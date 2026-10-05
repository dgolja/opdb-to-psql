package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/pgsql"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an OPDB JSON v2 export file into the database",
	}

	filePath := addFileFlag(cmd)
	_ = cmd.MarkFlagRequired("file")

	addDBURLFlag(cmd)

	truncate := cmd.Flags().Bool("truncate", false, "truncate all OPDB tables before importing")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		dbURL, err := dbURL(cmd)
		if err != nil {
			return err
		}
		return runImport(cmd.Context(), newLogger(cmd), *filePath, dbURL, *truncate)
	}
	return cmd
}

func runImport(ctx context.Context, log *slog.Logger, path, dbURL string, truncate bool) error {
	data, err := opdbv2.LoadFromFile(path)
	if err != nil {
		return err
	}
	// Checked before connecting: with --truncate an empty file (for example
	// "{}") would otherwise wipe the tables and load nothing.
	if len(data.Entries) == 0 {
		return fmt.Errorf("%s has no entries, nothing to import", path)
	}

	dbData, err := pgsql.New(ctx, data, dbURL)
	if err != nil {
		return err
	}
	dbData.WithLogger(log)

	defer dbData.CloseGracefully(ctx)

	log.InfoContext(ctx, "importing", "file", path, "entries", len(data.Entries))

	return dbData.LoadToDB(ctx, pgsql.LoadOptions{Truncate: truncate})
}

func init() {
	rootCmd.AddCommand(newImportCmd())
}
