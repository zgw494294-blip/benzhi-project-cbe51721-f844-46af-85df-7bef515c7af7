package tillseal

import (
	"errors"
	"fmt"
	"io/fs"
	"sync"
)

type Service struct {
	Store Store
	// mu serializes the persistence phase (reload-merge-save) of mutating
	// operations so that a concurrent count cannot overwrite another count's
	// newly persisted denomination. It is a pointer so that copies of Service
	// (for example when a value receiver is invoked) share a single lock and
	// stay mutually exclusive. The initial load is performed outside the lock
	// so concurrent reads of the ledger can proceed in parallel.
	mu *sync.Mutex
}

func NewService(store Store) Service {
	return Service{Store: store, mu: &sync.Mutex{}}
}

func (s Service) Open(id string, expectedCents int64, denominations []int64, toleranceCents int64) (Session, error) {
	session, err := OpenSession(id, expectedCents, denominations, toleranceCents)
	if err != nil {
		return Session{}, err
	}
	if err := s.persist(func(ledger Ledger) error {
		if _, exists := ledger.Sessions[id]; exists {
			return ErrDuplicateSession
		}
		ledger.Sessions[id] = session
		return nil
	}); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s Service) Count(id string, denomination, quantity int64) (Session, error) {
	// Load the session outside the lock so concurrent counts can both read the
	// ledger. Validation of the request happens here against this snapshot.
	_, session, err := s.loadSession(id)
	if err != nil {
		return Session{}, err
	}
	if err := session.AddCount(denomination, quantity); err != nil {
		return Session{}, err
	}
	// Persist by reloading the latest ledger under the lock and re-applying
	// the count, so a concurrent count's denomination is not overwritten.
	if err := s.persist(func(ledger Ledger) error {
		current, exists := ledger.Sessions[id]
		if !exists {
			return ErrSessionNotFound
		}
		if err := current.AddCount(denomination, quantity); err != nil {
			return err
		}
		ledger.Sessions[id] = current
		return nil
	}); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s Service) Reconcile(id string) (Session, error) {
	_, session, err := s.loadSession(id)
	if err != nil {
		return Session{}, err
	}
	if err := session.Reconcile(); err != nil {
		return Session{}, err
	}
	if err := s.persist(func(ledger Ledger) error {
		current, exists := ledger.Sessions[id]
		if !exists {
			return ErrSessionNotFound
		}
		if err := current.Reconcile(); err != nil {
			return err
		}
		ledger.Sessions[id] = current
		return nil
	}); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s Service) Show(id string) (Session, error) {
	_, session, err := s.loadSession(id)
	return session, err
}

// persist reloads the latest ledger, applies mutate to it, and saves the
// result, all while holding the service mutex. Reloading inside the lock is
// what prevents the lost-update problem: a concurrent count that wrote its
// denomination between this call's initial load and its save is observed and
// preserved instead of being clobbered.
func (s Service) persist(mutate func(Ledger) error) error {
	mu := s.lock()
	mu.Lock()
	defer mu.Unlock()

	ledger, err := s.loadOrCreate()
	if err != nil {
		return err
	}
	if err := mutate(ledger); err != nil {
		return err
	}
	return s.Store.Save(ledger)
}

// lock returns the shared mutex guarding the persistence phase against
// concurrent operations on the same service value. It is a pointer so that
// copies of Service (for example when a value receiver is invoked) share a
// single lock and stay mutually exclusive.
func (s Service) lock() *sync.Mutex {
	if s.mu != nil {
		return s.mu
	}
	// Defensive: a Service constructed without NewService has no guard.
	// Fall back to an isolated (per-call) mutex rather than panicking.
	return &sync.Mutex{}
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
