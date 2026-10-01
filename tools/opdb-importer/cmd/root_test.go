package cmd

import "testing"

// Both subcommands register --db-url. Each must read its own flag value.
func TestDBURLIsReadPerCommand(t *testing.T) {
	t.Setenv("DB_URL", "")
	imp, exp := newImportCmd(), newExportCmd() // created together, like in the real CLI

	if err := imp.ParseFlags([]string{"-f", "x.json", "-d", "postgresql://import"}); err != nil {
		t.Fatal(err)
	}
	if err := exp.ParseFlags([]string{"-d", "postgresql://export"}); err != nil {
		t.Fatal(err)
	}

	if got, err := dbURL(exp); err != nil || got != "postgresql://export" {
		t.Errorf("export: got %q, %v", got, err)
	}
	if got, err := dbURL(imp); err != nil || got != "postgresql://import" {
		t.Errorf("import: got %q, %v", got, err)
	}
}

func TestDBURLFallsBackToEnv(t *testing.T) {
	t.Setenv("DB_URL", "postgresql://from-env")
	cmd := newExportCmd()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := dbURL(cmd); err != nil || got != "postgresql://from-env" {
		t.Errorf("got %q, %v", got, err)
	}

	// the flag wins over the environment
	if err := cmd.ParseFlags([]string{"-d", "postgresql://flag"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := dbURL(cmd); got != "postgresql://flag" {
		t.Errorf("flag should win over env, got %q", got)
	}
}

func TestDBURLRequired(t *testing.T) {
	t.Setenv("DB_URL", "")
	cmd := newExportCmd()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := dbURL(cmd); err == nil {
		t.Fatal("expected an error when neither flag nor env is set")
	}
}
