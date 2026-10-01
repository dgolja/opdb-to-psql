package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The URL is unreachable: getting the "no entries" error means the import
// stopped before connecting, so --truncate could not have wiped anything.
func TestRunImportRejectsFileWithoutEntries(t *testing.T) {
	for name, content := range map[string]string{
		"empty object":  `{}`,
		"empty entries": `{"entries": []}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "empty.json")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			err := runImport(context.Background(), slog.Default(), path, "postgresql://127.0.0.1:1/none", true)
			if err == nil || !strings.Contains(err.Error(), "no entries") {
				t.Fatalf("expected a no entries error, got %v", err)
			}
		})
	}
}
