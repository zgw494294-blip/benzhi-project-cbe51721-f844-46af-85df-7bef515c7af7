package tillseal

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRoundTripAndMalformedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	store := NewStore(path)
	session, err := OpenSession("shift-a", 100, []int64{25}, 0)
	if err != nil {
		t.Fatal(err)
	}
	ledger := NewLedger()
	ledger.Sessions[session.ID] = session
	if err := store.Save(ledger); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != LedgerVersion || loaded.Sessions[session.ID].ID != session.ID {
		t.Fatalf("unexpected loaded ledger: %+v", loaded)
	}

	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "malformed ledger") {
		t.Fatalf("expected malformed-data error, got %v", err)
	}
}

func TestStoreRejectsInvalidLedger(t *testing.T) {
	ledger := NewLedger()
	ledger.Version = LedgerVersion + 1
	store := NewStore(filepath.Join(t.TempDir(), "ledger.json"))
	if err := store.Save(ledger); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestStoreReturnsPersistenceFailuresAndCleansTemporaryFile(t *testing.T) {
	cases := []struct {
		name      string
		writeErr  error
		syncErr   error
		closeErr  error
		renameErr error
	}{
		{name: "write", writeErr: errors.New("write sentinel")},
		{name: "sync", syncErr: errors.New("sync sentinel")},
		{name: "close", closeErr: errors.New("close sentinel")},
		{name: "rename", renameErr: errors.New("rename sentinel")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &failureFS{
				temp: &fakeTemp{
					name:     "/tmp/.ledger.json.tmp-test",
					writeErr: testCase.writeErr,
					syncErr:  testCase.syncErr,
					closeErr: testCase.closeErr,
				},
				renameErr: testCase.renameErr,
			}
			store := Store{Path: "/tmp/ledger.json", FS: fake}
			if err := store.Save(validLedger()); err == nil {
				t.Fatal("expected persistence error")
			} else {
				if testCase.writeErr != nil && !errors.Is(err, testCase.writeErr) {
					t.Fatalf("missing write error: %v", err)
				}
				if testCase.syncErr != nil && !errors.Is(err, testCase.syncErr) {
					t.Fatalf("missing sync error: %v", err)
				}
				if testCase.closeErr != nil && !errors.Is(err, testCase.closeErr) {
					t.Fatalf("missing close error: %v", err)
				}
				if testCase.renameErr != nil && !errors.Is(err, testCase.renameErr) {
					t.Fatalf("missing rename error: %v", err)
				}
			}
			if len(fake.removed) != 1 || fake.removed[0] != fake.temp.name {
				t.Fatalf("temporary file was not removed: %v", fake.removed)
			}
		})
	}
}

func TestStoreLoadReturnsReadAndCloseFailures(t *testing.T) {
	readErr := errors.New("read sentinel")
	closeErr := errors.New("close sentinel")
	store := Store{
		Path: "/tmp/ledger.json",
		FS:   &readFS{file: &fakeReadCloser{reader: errorReader{err: readErr}, closeErr: closeErr}},
	}
	if _, err := store.Load(); err == nil || !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Fatalf("expected read and close errors, got %v", err)
	}
}

func validLedger() Ledger {
	session, _ := OpenSession("shift-a", 100, []int64{25}, 0)
	ledger := NewLedger()
	ledger.Sessions[session.ID] = session
	return ledger
}

type fakeTemp struct {
	name     string
	writeErr error
	syncErr  error
	closeErr error
}

func (f *fakeTemp) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(data), nil
}

func (f *fakeTemp) Sync() error {
	return f.syncErr
}

func (f *fakeTemp) Close() error {
	return f.closeErr
}

func (f *fakeTemp) Name() string {
	return f.name
}

type failureFS struct {
	temp      *fakeTemp
	renameErr error
	removed   []string
}

func (f *failureFS) Open(string) (io.ReadCloser, error) {
	return nil, errors.New("unexpected open")
}

func (f *failureFS) CreateTemp(string, string) (TempFile, error) {
	return f.temp, nil
}

func (f *failureFS) Rename(string, string) error {
	return f.renameErr
}

func (f *failureFS) Remove(name string) error {
	f.removed = append(f.removed, name)
	return nil
}

type fakeReadCloser struct {
	reader   io.Reader
	closeErr error
}

func (f *fakeReadCloser) Read(data []byte) (int, error) {
	return f.reader.Read(data)
}

func (f *fakeReadCloser) Close() error {
	return f.closeErr
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

type readFS struct {
	file io.ReadCloser
}

func (f *readFS) Open(string) (io.ReadCloser, error) {
	return f.file, nil
}

func (f *readFS) CreateTemp(string, string) (TempFile, error) {
	return nil, errors.New("unexpected temp")
}

func (f *readFS) Rename(string, string) error {
	return errors.New("unexpected rename")
}

func (f *readFS) Remove(string) error {
	return errors.New("unexpected remove")
}
