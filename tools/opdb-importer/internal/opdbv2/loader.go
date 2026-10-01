package opdbv2

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LoadFromFile reads and parses an OPDB export JSON file from disk.
func LoadFromFile(path string) (*Export, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	return LoadFromReader(f)
}

// LoadFromReader parses OPDB export JSON from any io.Reader.
func LoadFromReader(r io.Reader) (*Export, error) {
	var export Export
	dec := json.NewDecoder(r)
	if err := dec.Decode(&export); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	// Unlike json.Unmarshal, Decode stops after the first value; reject trailing data.
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("parsing JSON: unexpected data after top-level value")
	}

	return &export, nil
}
