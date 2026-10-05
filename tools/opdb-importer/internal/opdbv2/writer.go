package opdbv2

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ErrFileExists is returned instead of overwriting an existing output file.
var ErrFileExists = errors.New("file already exists, not overwriting")

// EnsureFileNotExists returns ErrFileExists if path exists. Call it before
// doing expensive work whose result would be written to path.
func EnsureFileNotExists(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return ErrFileExists
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// WriteJSON writes the export as JSON to w.
func (e *Export) WriteJSON(w io.Writer) error {
	return json.NewEncoder(w).Encode(e)
}

// WriteFile writes the export as JSON to a temporary file next to path and then
// links it into place, so a failed write never leaves a partial file at path
// and an existing file is never overwritten (ErrFileExists is returned).
func (e *Export) WriteFile(path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if err := e.WriteJSON(tmp); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}

	// Link fails if path exists, which keeps the "never overwrite" guarantee
	// even if the file appeared while we were querying.
	if err := os.Link(tmp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}
		return err
	}
	return nil
}
