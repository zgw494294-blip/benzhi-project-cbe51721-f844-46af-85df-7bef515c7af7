package tillseal

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const LedgerVersion = 1

type Status string

const (
	StatusActive   Status = "active"
	StatusBalanced Status = "balanced"
	StatusVariance Status = "variance"
)

var (
	ErrValidation          = errors.New("validation failed")
	ErrSessionNotFound     = errors.New("session not found")
	ErrDuplicateSession    = errors.New("session already exists")
	ErrSessionClosed       = errors.New("session is already reconciled")
	ErrDuplicateCount      = errors.New("denomination was already counted")
	ErrUnknownDenomination = errors.New("denomination is not configured")
	ErrIncompleteSession   = errors.New("session is missing denomination counts")
)

type Ledger struct {
	Version  int                `json:"version"`
	Sessions map[string]Session `json:"sessions"`
}

type Session struct {
	ID             string          `json:"id"`
	ExpectedCents  int64           `json:"expected_cents"`
	Denominations  []int64         `json:"denominations_cents"`
	ToleranceCents int64           `json:"tolerance_cents"`
	Counts         map[int64]int64 `json:"counts"`
	Status         Status          `json:"status"`
	ActualCents    int64           `json:"actual_cents"`
	VarianceCents  int64           `json:"variance_cents"`
}

func NewLedger() Ledger {
	return Ledger{Version: LedgerVersion, Sessions: make(map[string]Session)}
}

func OpenSession(id string, expectedCents int64, denominations []int64, toleranceCents int64) (Session, error) {
	if id == "" {
		return Session{}, fmt.Errorf("%w: session id is required", ErrValidation)
	}
	if expectedCents < 0 {
		return Session{}, fmt.Errorf("%w: expected cents cannot be negative", ErrValidation)
	}
	if toleranceCents < 0 {
		return Session{}, fmt.Errorf("%w: tolerance cents cannot be negative", ErrValidation)
	}
	if len(denominations) == 0 {
		return Session{}, fmt.Errorf("%w: at least one denomination is required", ErrValidation)
	}

	denominations = append([]int64(nil), denominations...)
	sort.Slice(denominations, func(i, j int) bool { return denominations[i] < denominations[j] })
	for i, denomination := range denominations {
		if denomination <= 0 {
			return Session{}, fmt.Errorf("%w: denomination %d must be positive", ErrValidation, denomination)
		}
		if i > 0 && denominations[i-1] == denomination {
			return Session{}, fmt.Errorf("%w: denomination %d is duplicated", ErrValidation, denomination)
		}
	}

	return Session{
		ID:             id,
		ExpectedCents:  expectedCents,
		Denominations:  denominations,
		ToleranceCents: toleranceCents,
		Counts:         make(map[int64]int64),
		Status:         StatusActive,
	}, nil
}

func (l Ledger) Validate() error {
	if l.Version != LedgerVersion {
		return fmt.Errorf("%w: unsupported ledger version %d", ErrValidation, l.Version)
	}
	if l.Sessions == nil {
		return fmt.Errorf("%w: sessions must be an object", ErrValidation)
	}
	for id, session := range l.Sessions {
		if id != session.ID {
			return fmt.Errorf("%w: session key %q does not match id %q", ErrValidation, id, session.ID)
		}
		if err := session.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s Session) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("%w: session id is required", ErrValidation)
	}
	if s.ExpectedCents < 0 {
		return fmt.Errorf("%w: expected cents cannot be negative", ErrValidation)
	}
	if s.ToleranceCents < 0 {
		return fmt.Errorf("%w: tolerance cents cannot be negative", ErrValidation)
	}
	if len(s.Denominations) == 0 {
		return fmt.Errorf("%w: session %q has no denominations", ErrValidation, s.ID)
	}
	seen := make(map[int64]struct{}, len(s.Denominations))
	for _, denomination := range s.Denominations {
		if denomination <= 0 {
			return fmt.Errorf("%w: denomination %d must be positive", ErrValidation, denomination)
		}
		if _, exists := seen[denomination]; exists {
			return fmt.Errorf("%w: denomination %d is duplicated", ErrValidation, denomination)
		}
		seen[denomination] = struct{}{}
	}
	for denomination, quantity := range s.Counts {
		if _, exists := seen[denomination]; !exists {
			return fmt.Errorf("%w: %d is not configured", ErrValidation, denomination)
		}
		if quantity < 0 {
			return fmt.Errorf("%w: quantity for %d cannot be negative", ErrValidation, denomination)
		}
		if _, err := multiplyAndAdd(0, denomination, quantity); err != nil {
			return fmt.Errorf("%w: quantity for %d is too large", ErrValidation, denomination)
		}
	}

	switch s.Status {
	case StatusActive:
		if s.ActualCents != 0 || s.VarianceCents != 0 {
			return fmt.Errorf("%w: active session %q has receipt totals", ErrValidation, s.ID)
		}
		if s.complete() {
			if _, err := s.total(); err != nil {
				return err
			}
		}
	case StatusBalanced, StatusVariance:
		if !s.complete() {
			return fmt.Errorf("%w: reconciled session %q is incomplete", ErrValidation, s.ID)
		}
		actual, err := s.total()
		if err != nil {
			return err
		}
		variance := actual - s.ExpectedCents
		if s.ActualCents != actual || s.VarianceCents != variance {
			return fmt.Errorf("%w: receipt totals for %q do not match counts", ErrValidation, s.ID)
		}
		withinTolerance := variance <= s.ToleranceCents && variance >= -s.ToleranceCents
		if (s.Status == StatusBalanced) != withinTolerance {
			return fmt.Errorf("%w: receipt status for %q does not match variance", ErrValidation, s.ID)
		}
	default:
		return fmt.Errorf("%w: unknown status %q", ErrValidation, s.Status)
	}
	return nil
}

func (s Session) AddCount(denomination, quantity int64) error {
	if s.Status != StatusActive {
		return ErrSessionClosed
	}
	if quantity < 0 {
		return fmt.Errorf("%w: quantity cannot be negative", ErrValidation)
	}
	if !s.hasDenomination(denomination) {
		return ErrUnknownDenomination
	}
	if _, exists := s.Counts[denomination]; exists {
		return ErrDuplicateCount
	}
	if _, err := multiplyAndAdd(0, denomination, quantity); err != nil {
		return fmt.Errorf("%w: quantity is too large", ErrValidation)
	}
	s.Counts[denomination] = quantity
	return nil
}

func (s *Session) Reconcile() error {
	if s.Status != StatusActive {
		return ErrSessionClosed
	}
	if !s.complete() {
		return ErrIncompleteSession
	}
	actual, err := s.total()
	if err != nil {
		return err
	}
	s.ActualCents = actual
	s.VarianceCents = actual - s.ExpectedCents
	if s.VarianceCents <= s.ToleranceCents && s.VarianceCents >= -s.ToleranceCents {
		s.Status = StatusBalanced
	} else {
		s.Status = StatusVariance
	}
	return nil
}

func (s Session) complete() bool {
	return len(s.Counts) == len(s.Denominations)
}

func (s Session) hasDenomination(denomination int64) bool {
	for _, configured := range s.Denominations {
		if configured == denomination {
			return true
		}
	}
	return false
}

func (s Session) total() (int64, error) {
	var total int64
	for _, denomination := range s.Denominations {
		quantity := s.Counts[denomination]
		var err error
		total, err = multiplyAndAdd(total, denomination, quantity)
		if err != nil {
			return 0, fmt.Errorf("%w: total overflows cents", ErrValidation)
		}
	}
	return total, nil
}

func multiplyAndAdd(total, denomination, quantity int64) (int64, error) {
	if quantity != 0 && denomination > math.MaxInt64/quantity {
		return 0, errors.New("integer overflow")
	}
	product := denomination * quantity
	if product > math.MaxInt64-total {
		return 0, errors.New("integer overflow")
	}
	return total + product, nil
}
