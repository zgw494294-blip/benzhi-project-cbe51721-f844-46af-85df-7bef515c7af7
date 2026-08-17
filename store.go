package tillseal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type TempFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

type FileSystem interface {
	Open(name string) (io.ReadCloser, error)
	CreateTemp(dir, pattern string) (TempFile, error)
	Rename(oldPath, newPath string) error
	Remove(name string) error
}

type OSFileSystem struct{}

func (OSFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

func (OSFileSystem) CreateTemp(dir, pattern string) (TempFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (OSFileSystem) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

type Store struct {
	Path string
	FS   FileSystem
}

func NewStore(path string) Store {
	return Store{Path: path, FS: OSFileSystem{}}
}

func (s Store) Load() (Ledger, error) {
	if s.Path == "" {
		return Ledger{}, fmt.Errorf("%w: ledger path is required", ErrValidation)
	}
	fs := s.filesystem()
	file, err := fs.Open(s.Path)
	if err != nil {
		return Ledger{}, fmt.Errorf("read ledger: %w", err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return Ledger{}, fmt.Errorf("read ledger: %w", errors.Join(readErr, closeErr))
	}

	var ledger Ledger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return Ledger{}, fmt.Errorf("malformed ledger: %w", err)
	}
	if err := ledger.Validate(); err != nil {
		return Ledger{}, fmt.Errorf("load ledger: %w", err)
	}
	return ledger, nil
}

func (s Store) Save(ledger Ledger) error {
	if s.Path == "" {
		return fmt.Errorf("%w: ledger path is required", ErrValidation)
	}
	if err := ledger.Validate(); err != nil {
		return fmt.Errorf("save ledger: %w", err)
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("write ledger: %w", err)
	}
	data = append(data, '\n')

	fs := s.filesystem()
	dir := filepath.Dir(s.Path)
	base := filepath.Base(s.Path)
	temp, err := fs.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("write ledger: %w", err)
	}
	tempPath := temp.Name()

	fail := func(cause error) error {
		closeErr := temp.Close()
		removeErr := fs.Remove(tempPath)
		return fmt.Errorf("%w", errors.Join(cause, closeErr, removeErr))
	}

	if err := writeAll(temp, data); err != nil {
		return fail(fmt.Errorf("write ledger: %w", err))
	}
	if err := temp.Sync(); err != nil {
		return fail(fmt.Errorf("sync ledger: %w", err))
	}
	if err := temp.Close(); err != nil {
		removeErr := fs.Remove(tempPath)
		return fmt.Errorf("%w", errors.Join(fmt.Errorf("close ledger: %w", err), removeErr))
	}
	if err := fs.Rename(tempPath, s.Path); err != nil {
		removeErr := fs.Remove(tempPath)
		return fmt.Errorf("%w", errors.Join(fmt.Errorf("rename ledger: %w", err), removeErr))
	}
	return nil
}

func (s Store) filesystem() FileSystem {
	if s.FS == nil {
		return OSFileSystem{}
	}
	return s.FS
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
