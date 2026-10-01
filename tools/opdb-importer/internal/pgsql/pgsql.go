package pgsql

import (
	"context"
	"log/slog"
	"strings"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	tableManufacturers = "manufacturers"
	tablePeople        = "people"
	tableImages        = "images"
	tableImageVariants = "image_variants"
	tableFeatures      = "features"
	tableOPDB          = "opdb"
	tableOPDBPeople    = "opdb_people"
	tableOPDBFeatures  = "opdb_features"
	tableOPDBImages    = "opdb_images"
)

// allTables lists every table the importer manages, in the same order as the
// TRUNCATE in supabase/seed.sql.
var allTables = []string{
	tableOPDBPeople, tablePeople, tableOPDBImages, tableImages, tableImageVariants,
	tableFeatures, tableOPDBFeatures, tableOPDB, tableManufacturers,
}

// DB is the subset of *pgx.Conn the importer uses. It exists so tests can
// substitute a mock connection.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Close(ctx context.Context) error
}

type OPDBv2 struct {
	Data *opdbv2.Export
	conn DB
	log  *slog.Logger
}

// WithLogger sets the logger used for progress (Info) and for every SQL
// statement with its arguments (Debug). By default nothing is logged. Calling
// it again replaces the previous logger.
func (i *OPDBv2) WithLogger(log *slog.Logger) *OPDBv2 {
	// don't wrap a wrapper: statements would be logged once per call
	conn := i.conn
	if l, ok := conn.(*loggingDB); ok {
		conn = l.DB
	}

	i.log = log
	i.conn = &loggingDB{DB: conn, log: log}
	return i
}

// NewWithDB returns an OPDBv2 that uses an already established connection.
func NewWithDB(db DB, data *opdbv2.Export) *OPDBv2 {
	return &OPDBv2{Data: data, conn: db, log: slog.New(slog.DiscardHandler)}
}

// New connects to PostgreSQL at dbURL and returns an OPDBv2 bound to data.
// Call Close when done.
func New(ctx context.Context, data *opdbv2.Export, dbURL string) (*OPDBv2, error) {

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, err
	}

	return NewWithDB(conn, data), nil
}

// truncateTables empties every table listed in allTables and resets their
// identity sequences (same as the TRUNCATE in supabase/seed.sql). It runs on
// tx, so it is undone if the surrounding import fails.
func truncateTables(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx,
		"TRUNCATE TABLE "+strings.Join(allTables, ", ")+" RESTART IDENTITY")
	return err
}

// Close releases the database connection.
func (i OPDBv2) Close(ctx context.Context) error {
	return i.conn.Close(ctx)
}
