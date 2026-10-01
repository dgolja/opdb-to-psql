package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "opdb-importer",
	Short: "Tools for importing and exporting matchplay OPDB pinball v2 format to PostgreSQL",
}

func init() {
	rootCmd.PersistentFlags().Bool("debug", false, "log every SQL statement and its arguments to stderr")
}

// newLogger returns a logger writing to stderr (stdout may carry exported
// JSON). It logs at Debug level when --debug is set, otherwise at Info.
func newLogger(cmd *cobra.Command) *slog.Logger {
	level := slog.LevelInfo
	if debug, _ := cmd.Flags().GetBool("debug"); debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// Execute runs the root command and exits with status 1 on error.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// addFileFlag registers a shared --file/-f flag on a command
// and returns a pointer to the bound string value.
func addFileFlag(cmd *cobra.Command) *string {
	var filePath string
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "path to the OPDB JSON export file")
	return &filePath
}

// addDBURLFlag registers a shared --db-url/-d flag on a command. Read it with
// dbURL, which also falls back to the DB_URL environment variable.
func addDBURLFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("db-url", "d", "", "Valid PostgreSQL url (example: postgresql://postgres:postgres@127.0.0.1:54322/postgres)")
}

// dbURL returns the database URL for cmd: the --db-url flag if set, otherwise
// the DB_URL environment variable. It is an error if neither is set.
func dbURL(cmd *cobra.Command) (string, error) {
	url, err := cmd.Flags().GetString("db-url")
	if err != nil {
		return "", err
	}
	if url == "" {
		url = os.Getenv("DB_URL")
	}
	if url == "" {
		return "", errors.New("db-url is required: set --db-url or the DB_URL environment variable")
	}
	return url, nil
}
