# TillSeal

TillSeal is a small Go command-line tool for closing a cash drawer by denomination. It records one active session in a versioned local JSON ledger, accepts one quantity for every configured denomination, and turns a complete count into a balanced or variance receipt.

The application uses only the Go standard library and requires Go 1.22.0 or newer.

## Commands

All monetary values are integer cents. The ledger path defaults to `tillseal.json` and can be changed with `--ledger`.

```text
tillseal open --id shift-2026-08-17 --expected-cents 1275 --denominations-cents 5,10,25,100,500 --tolerance-cents 0
tillseal count --id shift-2026-08-17 --denomination-cents 5 --quantity 1
tillseal count --id shift-2026-08-17 --denomination-cents 10 --quantity 2
tillseal count --id shift-2026-08-17 --denomination-cents 25 --quantity 4
tillseal count --id shift-2026-08-17 --denomination-cents 100 --quantity 1
tillseal count --id shift-2026-08-17 --denomination-cents 500 --quantity 1
tillseal reconcile --id shift-2026-08-17
tillseal show --id shift-2026-08-17
```

An omitted tolerance is zero. A session is balanced when the absolute difference between its counted total and expected total is within that tolerance. Once reconciled, the session is an immutable receipt.

The `smoke` command runs the complete workflow against a temporary ledger and removes it before exiting:

```text
go run ./cmd/tillseal smoke
```

## Ledger safety

Every save validates the complete versioned document, writes beside the target ledger, syncs and closes the temporary file, then replaces the target with an atomic rename. Any failed write, sync, close, or rename removes the temporary file and returns the failure.

## Development

```text
go test ./...
go run ./cmd/tillseal smoke
```
