package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
)

func TestRunExportFailsBeforeConnectingWhenFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// the URL is not reachable: getting ErrFileExists means we never tried to connect
	err := runExport(context.Background(), slog.Default(), path, "postgresql://127.0.0.1:1/none")
	if !errors.Is(err, opdbv2.ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got %v", err)
	}
}
