package tillseal

import (
	"errors"
	"fmt"
	"io/fs"
)

type Service struct {
	Store Store
}

func NewService(store Store) Service {
	return Service{Store: store}
}

func (s Service) Open(id string, expectedCents int64, denominations []int64, toleranceCents int64) (Session, error) {
	session, err := OpenSession(id, expectedCents, denominations, toleranceCents)
	if err != nil {
		return Session{}, err
	}
	ledger, err := s.loadOrCreate()
	if err != nil {
		return Session{}, err
	}
	if _, exists := ledger.Sessions[id]; exists {
		return Session{}, ErrDuplicateSession
	}
	ledger.Sessions[id] = session
	if err := s.Store.Save(ledger); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s Service) Count(id string, denomination, quantity int64) (Session, error) {
	ledger, session, err := s.loadSession(id)
	if err != nil {
		return Session{}, err
	}
	if err := session.AddCount(denomination, quantity); err != nil {
		return Session{}, err
	}
	ledger.Sessions[id] = session
	if err := s.Store.Save(ledger); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s Service) Reconcile(id string) (Session, error) {
	ledger, session, err := s.loadSession(id)
	if err != nil {
		return Session{}, err
	}
	if err := session.Reconcile(); err != nil {
		return Session{}, err
	}
	ledger.Sessions[id] = session
	if err := s.Store.Save(ledger); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s Service) Show(id string) (Session, error) {
	_, session, err := s.loadSession(id)
	return session, err
}

func (s Service) loadSession(id string) (Ledger, Session, error) {
	if id == "" {
		return Ledger{}, Session{}, fmt.Errorf("%w: session id is required", ErrValidation)
	}
	ledger, err := s.Store.Load()
	if err != nil {
		return Ledger{}, Session{}, err
	}
	session, exists := ledger.Sessions[id]
	if !exists {
		return Ledger{}, Session{}, ErrSessionNotFound
	}
	return ledger, session, nil
}

func (s Service) loadOrCreate() (Ledger, error) {
	ledger, err := s.Store.Load()
	if err == nil {
		return ledger, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return NewLedger(), nil
	}
	return Ledger{}, err
}
