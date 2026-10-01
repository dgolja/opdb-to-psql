//go:build integration

// Integration tests run against a real PostgreSQL database. They need
// TEST_DB_URL, for example the local Supabase database:
//
//	TEST_DB_URL=postgresql://postgres:postgres@127.0.0.1:54322/postgres \
//	  go test -tags integration ./internal/pgsql
//
// TEST_DATA_FILE optionally points at a different OPDB JSON export than the
// default testdata/small-sample-v2.json (a relative path is relative to this
// directory, so absolute paths are easier).
//
// TEST_KEEP_SCHEMA=1 keeps the temporary schemas (with their data) after the
// tests, so a failure can be inspected. Run with -v to see the schema names.
//
// Each test creates its own schema, applies supabase/migrations into it and
// drops it afterwards, so existing data in the database is never touched.
package pgsql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
)

const (
	defaultSamplePath = "../../testdata/opdb-v2.sample.json"
	migrationsGlob    = "../../../../supabase/migrations/*.sql"
)

// samplePath returns the export file the tests import.
func samplePath() string {
	if p := os.Getenv("TEST_DATA_FILE"); p != "" {
		return p
	}
	return defaultSamplePath
}

// newTestDB creates an isolated schema, applies the table migrations to it and
// returns a connection whose search_path points at that schema.
func newTestDB(t *testing.T) *pgx.Conn {
	t.Helper()

	url := os.Getenv("TEST_DB_URL")
	if url == "" {
		t.Skip("TEST_DB_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to TEST_DB_URL: %v", err)
	}

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	schema := "it_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		conn.Close(ctx)
		if os.Getenv("TEST_KEEP_SCHEMA") != "" {
			t.Logf("keeping schema %s (TEST_KEEP_SCHEMA is set); inspect with: SET search_path = %s;", schema, schema)
			admin.Close(ctx)
			return
		}
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("dropping schema %s: %v", schema, err)
		}
		admin.Close(ctx)
	})

	files, err := filepath.Glob(migrationsGlob)
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	sort.Strings(files)
	for _, f := range files {
		// the export view/RPC layer is for PostgREST and not used by the importer
		if strings.HasSuffix(f, "_func.sql") {
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("applying %s: %v", filepath.Base(f), err)
		}
	}
	return conn
}

func loadSample(t *testing.T) *opdbv2.Export {
	t.Helper()
	data, err := opdbv2.LoadFromFile(samplePath())
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Entries) == 0 {
		t.Fatalf("%s has no entries", samplePath())
	}
	return data
}

func count(t *testing.T, conn *pgx.Conn, table string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// expectedCounts computes, from the input alone, how many rows each table should hold.
func expectedCounts(entries []opdbv2.OPDB) map[string]int {
	want := map[string]int{
		tableManufacturers: len(uniqueManufacturers(entries)),
		tablePeople:        len(uniquePeople(entries)),
		tableFeatures:      len(uniqueFeatures(entries)),
		tableImages:        len(uniqueImages(entries)),
		tableOPDB:          len(entries),
	}
	for _, img := range uniqueImages(entries) {
		for _, url := range []string{img.URLs.Small, img.URLs.Medium, img.URLs.Large} {
			if url != "" {
				want[tableImageVariants]++
			}
		}
	}
	for _, e := range entries {
		want[tableOPDBPeople] += len(e.People)
		want[tableOPDBFeatures] += len(e.Features)
		want[tableOPDBImages] += len(e.Images)
	}
	return want
}

func assertCounts(t *testing.T, conn *pgx.Conn, want map[string]int) {
	t.Helper()
	for _, table := range allTables {
		if got := count(t, conn, table); got != want[table] {
			t.Errorf("%s: got %d rows, want %d", table, got, want[table])
		}
	}
}

// normalize makes two exports comparable: the database does not preserve the
// order of entries or of the nested lists, and nil and empty lists mean the same.
func normalize(e *opdbv2.Export) {
	sort.Slice(e.Entries, func(a, b int) bool { return e.Entries[a].OPDBID < e.Entries[b].OPDBID })
	for idx := range e.Entries {
		entry := &e.Entries[idx]
		sort.Slice(entry.People, func(a, b int) bool {
			if entry.People[a].Index != entry.People[b].Index {
				return entry.People[a].Index < entry.People[b].Index
			}
			return entry.People[a].OpdbPersonID < entry.People[b].OpdbPersonID
		})
		sort.Slice(entry.Features, func(a, b int) bool { return entry.Features[a].FeatureID < entry.Features[b].FeatureID })
		sort.Slice(entry.Images, func(a, b int) bool { return entry.Images[a].Group < entry.Images[b].Group })
		sort.Strings(entry.Keywords)
		if entry.People == nil {
			entry.People = []opdbv2.Person{}
		}
		if entry.Features == nil {
			entry.Features = []opdbv2.Feature{}
		}
		if entry.Images == nil {
			entry.Images = []opdbv2.Image{}
		}
		if entry.Keywords == nil {
			entry.Keywords = []string{}
		}
	}
}

func exportFromDB(t *testing.T, conn *pgx.Conn) *opdbv2.Export {
	t.Helper()
	db := NewWithDB(conn, &opdbv2.Export{})
	if err := db.LoadFromDB(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db.Data
}

func TestImportExport(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	input := loadSample(t)

	if err := NewWithDB(conn, input).LoadToDB(ctx, LoadOptions{}); err != nil {
		t.Fatalf("import: %v", err)
	}

	t.Run("row counts", func(t *testing.T) {
		assertCounts(t, conn, expectedCounts(input.Entries))
	})

	t.Run("round trip", func(t *testing.T) {
		got := exportFromDB(t, conn)
		want := loadSample(t) // fresh copy, the import above did not modify it but be safe
		normalize(got)
		normalize(want)

		timeEqual := cmp.Options{
			cmp.Comparer(func(a, b opdbv2.Date) bool { return a.Time.Equal(b.Time) }),
			cmp.Comparer(func(a, b opdbv2.Timestamp) bool { return a.Time.Equal(b.Time) }),
		}
		if diff := cmp.Diff(want, got, timeEqual); diff != "" {
			t.Errorf("export differs from the imported file (-input +export):\n%s", diff)
		}
	})

	t.Run("importing again changes nothing", func(t *testing.T) {
		if err := NewWithDB(conn, loadSample(t)).LoadToDB(ctx, LoadOptions{}); err != nil {
			t.Fatalf("second import: %v", err)
		}
		assertCounts(t, conn, expectedCounts(input.Entries))
	})

	t.Run("truncate removes everything", func(t *testing.T) {
		// a row that is not in the file must be gone after the truncate
		if _, err := conn.Exec(ctx, `INSERT INTO manufacturers (manufacturer_id, name, full_name) VALUES (32000, 'junk', 'junk')`); err != nil {
			t.Fatal(err)
		}
		if err := NewWithDB(conn, loadSample(t)).LoadToDB(ctx, LoadOptions{Truncate: true}); err != nil {
			t.Fatalf("import with truncate: %v", err)
		}
		assertCounts(t, conn, expectedCounts(input.Entries))
		var junk int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM manufacturers WHERE manufacturer_id = 32000`).Scan(&junk); err != nil {
			t.Fatal(err)
		}
		if junk != 0 {
			t.Error("row that is not in the file survived the truncate")
		}
	})
}

// The import is a single transaction: a failure part way leaves the database
// exactly as it was, including when --truncate was requested.
func TestFailedImportChangesNothing(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	good := loadSample(t)
	if err := NewWithDB(conn, good).LoadToDB(ctx, LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	want := expectedCounts(good.Entries)

	for name, opts := range map[string]LoadOptions{"plain": {}, "truncate": {Truncate: true}} {
		t.Run(name, func(t *testing.T) {
			bad := loadSample(t)
			bad.Entries[len(bad.Entries)-1].OPDBID = "not-a-valid-id" // violates chk_opdb_opdb_id

			if err := NewWithDB(conn, bad).LoadToDB(ctx, opts); err == nil {
				t.Fatal("expected the import to fail")
			}
			assertCounts(t, conn, want)
		})
	}

	t.Run("into empty tables", func(t *testing.T) {
		empty := newTestDB(t)
		bad := loadSample(t)
		bad.Entries[len(bad.Entries)-1].OPDBID = "not-a-valid-id"
		if err := NewWithDB(empty, bad).LoadToDB(ctx, LoadOptions{}); err == nil {
			t.Fatal("expected the import to fail")
		}
		assertCounts(t, empty, map[string]int{}) // all zero: parents were rolled back too
	})
}
