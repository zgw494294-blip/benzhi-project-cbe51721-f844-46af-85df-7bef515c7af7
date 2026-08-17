package tillseal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func RunSmoke(w io.Writer) error {
	directory, err := os.MkdirTemp("", "tillseal-smoke-")
	if err != nil {
		return fmt.Errorf("create smoke workspace: %w", err)
	}
	defer os.RemoveAll(directory)

	path := filepath.Join(directory, "ledger.json")
	service := NewService(NewStore(path))
	session, err := service.Open("market-stall-evening", 725, []int64{5, 10, 25, 100, 500}, 0)
	if err != nil {
		return fmt.Errorf("smoke open: %w", err)
	}
	counts := map[int64]int64{5: 1, 10: 2, 25: 4, 100: 1, 500: 1}
	for _, denomination := range session.Denominations {
		if _, err := service.Count(session.ID, denomination, counts[denomination]); err != nil {
			return fmt.Errorf("smoke count %d: %w", denomination, err)
		}
	}
	receipt, err := service.Reconcile(session.ID)
	if err != nil {
		return fmt.Errorf("smoke reconcile: %w", err)
	}
	if receipt.Status != StatusBalanced || receipt.ActualCents != 725 {
		return fmt.Errorf("smoke receipt did not balance")
	}
	if _, err := service.Show(session.ID); err != nil {
		return fmt.Errorf("smoke show: %w", err)
	}
	if _, err := fmt.Fprintf(w, "smoke ok session=%s status=%s actual_cents=%d\n", receipt.ID, receipt.Status, receipt.ActualCents); err != nil {
		return err
	}
	return nil
}
