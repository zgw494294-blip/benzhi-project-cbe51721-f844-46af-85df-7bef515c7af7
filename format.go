package tillseal

import (
	"fmt"
	"io"
)

func WriteSession(w io.Writer, session Session) error {
	if _, err := fmt.Fprintf(w, "session=%s\nstatus=%s\nexpected_cents=%d\nactual_cents=%d\nvariance_cents=%d\ntolerance_cents=%d\ndenominations_cents=", session.ID, session.Status, session.ExpectedCents, session.ActualCents, session.VarianceCents, session.ToleranceCents); err != nil {
		return err
	}
	for i, denomination := range session.Denominations {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, denomination); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	for _, denomination := range session.Denominations {
		if _, err := fmt.Fprintf(w, "count_%d=%d\n", denomination, session.Counts[denomination]); err != nil {
			return err
		}
	}
	return nil
}
