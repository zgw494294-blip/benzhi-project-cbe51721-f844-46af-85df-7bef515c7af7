package tillseal

import (
	"errors"
	"reflect"
	"testing"
)

func TestSessionLifecycle(t *testing.T) {
	store := NewStore(t.TempDir() + "/ledger.json")
	service := NewService(store)
	session, err := service.Open("shift-a", 725, []int64{500, 25, 5, 100, 10}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != StatusActive || !reflect.DeepEqual(session.Denominations, []int64{5, 10, 25, 100, 500}) {
		t.Fatalf("unexpected opened session: %+v", session)
	}

	if _, err := service.Reconcile(session.ID); !errors.Is(err, ErrIncompleteSession) {
		t.Fatalf("expected incomplete error, got %v", err)
	}
	if _, err := service.Count(session.ID, 5, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Count(session.ID, 5, 1); !errors.Is(err, ErrDuplicateCount) {
		t.Fatalf("expected duplicate count error, got %v", err)
	}
	if _, err := service.Count(session.ID, 7, 1); !errors.Is(err, ErrUnknownDenomination) {
		t.Fatalf("expected unknown denomination error, got %v", err)
	}
	for denomination, quantity := range map[int64]int64{10: 2, 25: 4, 100: 1, 500: 1} {
		if _, err := service.Count(session.ID, denomination, quantity); err != nil {
			t.Fatal(err)
		}
	}

	receipt, err := service.Reconcile(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != StatusBalanced || receipt.ActualCents != 725 || receipt.VarianceCents != 0 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if _, err := service.Reconcile(session.ID); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("expected repeated reconcile error, got %v", err)
	}
	if _, err := service.Count(session.ID, 5, 1); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("expected closed count error, got %v", err)
	}

	loaded, err := service.Show(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, receipt) {
		t.Fatalf("stored receipt differs\nwant: %+v\ngot:  %+v", receipt, loaded)
	}
}

func TestVarianceTolerance(t *testing.T) {
	store := NewStore(t.TempDir() + "/ledger.json")
	service := NewService(store)
	if _, err := service.Open("strict", 100, []int64{25}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Count("strict", 25, 5); err != nil {
		t.Fatal(err)
	}
	strict, err := service.Reconcile("strict")
	if err != nil {
		t.Fatal(err)
	}
	if strict.Status != StatusVariance || strict.VarianceCents != 25 {
		t.Fatalf("unexpected strict receipt: %+v", strict)
	}

	if _, err := service.Open("lenient", 100, []int64{25}, 25); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Count("lenient", 25, 5); err != nil {
		t.Fatal(err)
	}
	lenient, err := service.Reconcile("lenient")
	if err != nil {
		t.Fatal(err)
	}
	if lenient.Status != StatusBalanced || lenient.ToleranceCents != 25 {
		t.Fatalf("unexpected lenient receipt: %+v", lenient)
	}
}

func TestOpenValidation(t *testing.T) {
	for name, denominations := range map[string][]int64{
		"empty":       nil,
		"duplicate":   {5, 5},
		"nonpositive": {5, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := OpenSession(name, 0, denominations, 0); !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
	if _, err := OpenSession("negative-tolerance", 0, []int64{1}, -1); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected tolerance validation error, got %v", err)
	}
}
