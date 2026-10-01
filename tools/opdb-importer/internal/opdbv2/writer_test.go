package opdbv2

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := (&Export{}).WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "{\"entries\":null}\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	if err := (&Export{}).WriteFile(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "{\"entries\":null}\n" {
		t.Fatalf("got %q, %v", got, err)
	}

	// no temp files left behind
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("expected only out.json, got %v", entries)
	}

	// second write must not overwrite
	if err := (&Export{Entries: []OPDB{{}}}).WriteFile(path); !errors.Is(err, ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got %v", err)
	}
	if again, _ := os.ReadFile(path); string(again) != string(got) {
		t.Fatal("existing file was modified")
	}
}

func TestEnsureFileNotExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	if err := EnsureFileNotExists(path); err != nil {
		t.Fatalf("missing file should be fine, got %v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureFileNotExists(path); !errors.Is(err, ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got %v", err)
	}
}
