package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/pgsql"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "export data from the database to OPDB V2 json file. Can be used to verify data or backup",
	}

	filePath := addFileFlag(cmd)
	addDBURLFlag(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		dbURL, err := dbURL(cmd)
		if err != nil {
			return err
		}
		return runExport(cmd.Context(), newLogger(cmd), *filePath, dbURL)
	}
	return cmd
}

func runExport(ctx context.Context, log *slog.Logger, path, dbURL string) error {
	// fail before connecting or running any query if the result can't be written
	if path != "" {
		if err := opdbv2.EnsureFileNotExists(path); err != nil {
			return err
		}
	}

	dbData, err := pgsql.New(ctx, &opdbv2.Export{}, dbURL)
	if err != nil {
		return err
	}
	dbData.WithLogger(log)
	defer dbData.Close(context.Background())

	if err := dbData.LoadFromDB(ctx); err != nil {
		return err
	}

	// no file defined stdout it is
	if path == "" {
		return dbData.Data.WriteJSON(os.Stdout)
	}
	return dbData.Data.WriteFile(path)
}

func init() {
	rootCmd.AddCommand(newExportCmd())
}
