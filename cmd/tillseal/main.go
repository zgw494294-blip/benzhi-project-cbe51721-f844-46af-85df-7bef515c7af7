package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"tillseal"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	var err error
	switch args[0] {
	case "open":
		err = runOpen(args[1:], stdout)
	case "count":
		err = runCount(args[1:], stdout)
	case "reconcile":
		err = runReconcile(args[1:], stdout)
	case "show":
		err = runShow(args[1:], stdout)
	case "smoke":
		err = runSmoke(args[1:], stdout)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runOpen(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ledgerPath := fs.String("ledger", "tillseal.json", "path to the local ledger")
	id := fs.String("id", "", "unique session identifier")
	expected := fs.String("expected-cents", "", "expected drawer total in cents")
	denominations := fs.String("denominations-cents", "", "comma-separated positive denomination values in cents")
	tolerance := fs.String("tolerance-cents", "0", "allowed absolute variance in cents")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("open does not accept positional arguments")
	}
	expectedCents, err := parseRequiredInt64("expected-cents", *expected)
	if err != nil {
		return err
	}
	toleranceCents, err := parseInt64("tolerance-cents", *tolerance)
	if err != nil {
		return err
	}
	denominationValues, err := parseDenominations(*denominations)
	if err != nil {
		return err
	}
	session, err := tillseal.NewService(tillseal.NewStore(*ledgerPath)).Open(*id, expectedCents, denominationValues, toleranceCents)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "opened session=%s expected_cents=%d denominations_cents=%s tolerance_cents=%d\n", session.ID, session.ExpectedCents, joinInt64(session.Denominations), session.ToleranceCents)
	return err
}

func runCount(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("count", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ledgerPath := fs.String("ledger", "tillseal.json", "path to the local ledger")
	id := fs.String("id", "", "session identifier")
	denomination := fs.String("denomination-cents", "", "denomination value in cents")
	quantity := fs.String("quantity", "", "non-negative quantity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("count does not accept positional arguments")
	}
	denominationValue, err := parseRequiredInt64("denomination-cents", *denomination)
	if err != nil {
		return err
	}
	quantityValue, err := parseRequiredInt64("quantity", *quantity)
	if err != nil {
		return err
	}
	session, err := tillseal.NewService(tillseal.NewStore(*ledgerPath)).Count(*id, denominationValue, quantityValue)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "counted session=%s denomination_cents=%d quantity=%d\n", session.ID, denominationValue, quantityValue)
	return err
}

func runReconcile(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ledgerPath := fs.String("ledger", "tillseal.json", "path to the local ledger")
	id := fs.String("id", "", "session identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("reconcile does not accept positional arguments")
	}
	session, err := tillseal.NewService(tillseal.NewStore(*ledgerPath)).Reconcile(*id)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "reconciled session=%s status=%s expected_cents=%d actual_cents=%d variance_cents=%d tolerance_cents=%d\n", session.ID, session.Status, session.ExpectedCents, session.ActualCents, session.VarianceCents, session.ToleranceCents); err != nil {
		return err
	}
	return nil
}

func runShow(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ledgerPath := fs.String("ledger", "tillseal.json", "path to the local ledger")
	id := fs.String("id", "", "session identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("show does not accept positional arguments")
	}
	session, err := tillseal.NewService(tillseal.NewStore(*ledgerPath)).Show(*id)
	if err != nil {
		return err
	}
	return tillseal.WriteSession(stdout, session)
}

func runSmoke(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("smoke does not accept arguments")
	}
	return tillseal.RunSmoke(stdout)
}

func parseRequiredInt64(name, value string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("--%s is required", name)
	}
	return parseInt64(name, value)
}

func parseInt64(name, value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("--%s must be an integer: %w", name, err)
	}
	return parsed, nil
}

func parseDenominations(value string) ([]int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("--denominations-cents is required")
	}
	parts := strings.Split(value, ",")
	denominations := make([]int64, 0, len(parts))
	for _, part := range parts {
		denomination, err := parseInt64("denominations-cents", part)
		if err != nil {
			return nil, err
		}
		denominations = append(denominations, denomination)
	}
	return denominations, nil
}

func joinInt64(values []int64) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.FormatInt(value, 10)
	}
	return strings.Join(parts, ",")
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: tillseal <open|count|reconcile|show|smoke> [flags]")
	fmt.Fprintln(w, "ledger defaults to tillseal.json; all monetary values are integer cents")
}
