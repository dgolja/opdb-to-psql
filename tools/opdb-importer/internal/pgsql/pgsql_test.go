package pgsql

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/dgolja/opdb-to-psql/tools/opdb-importer/internal/opdbv2"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func newMock(t *testing.T) pgxmock.PgxConnIface {
	t.Helper()
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	return mock
}

// inTx runs one insert step inside a transaction, the way LoadToDB does.
func inTx(db *OPDBv2, step func(context.Context, pgx.Tx) ([]*tally, error)) ([]*tally, error) {
	ctx := context.Background()
	var tallies []*tally
	err := pgx.BeginFunc(ctx, db.conn, func(tx pgx.Tx) error {
		var err error
		tallies, err = step(ctx, tx)
		return err
	})
	return tallies, err
}

// expectCommit expects a commit followed by the deferred Rollback that
// pgx.BeginFunc always issues (a no-op on a real, already committed tx).
func expectCommit(mock pgxmock.PgxConnIface) {
	mock.ExpectCommit()
	mock.ExpectRollback().WillReturnError(pgx.ErrTxClosed)
}

func done(t *testing.T, mock pgxmock.PgxConnIface) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sampleEntries() []opdbv2.OPDB {
	bally := &opdbv2.Manufacturer{ManufacturerID: 8, Name: "Bally", FullName: "Bally Manufacturing Co."}
	feature := opdbv2.Feature{FeatureID: 1, Name: "Ramp", Group: "playfield"}
	person := opdbv2.Person{OpdbPersonID: 3, Name: "Jane", Role: "Design", Index: 0}
	img := opdbv2.Image{
		Group: "img1", Primary: true, Type: "backglass",
		URLs:  opdbv2.URLs{Large: "http://x/l.jpg", Small: "http://x/s.jpg"},
		Sizes: opdbv2.Sizes{Large: opdbv2.Size{Width: 10, Height: 20}, Small: opdbv2.Size{Width: 1, Height: 2}},
	}
	// two entries sharing every related record
	return []opdbv2.OPDB{
		{OPDBID: "A", Manufacturer: bally, Features: []opdbv2.Feature{feature}, People: []opdbv2.Person{person}, Images: []opdbv2.Image{img}},
		{OPDBID: "B", Manufacturer: bally, Features: []opdbv2.Feature{feature}, People: []opdbv2.Person{person}, Images: []opdbv2.Image{img}},
	}
}

func TestUniqueFunctions(t *testing.T) {
	entries := sampleEntries()
	entries = append(entries, opdbv2.OPDB{OPDBID: "C"}) // no manufacturer, no relations

	if got := uniqueManufacturers(entries); len(got) != 1 || got[8].Name != "Bally" {
		t.Errorf("manufacturers: %+v", got)
	}
	if got := uniquePeople(entries); len(got) != 1 || got[3].Name != "Jane" {
		t.Errorf("people: %+v", got)
	}
	if got := uniqueFeatures(entries); len(got) != 1 || got[1].Name != "Ramp" {
		t.Errorf("features: %+v", got)
	}
	if got := uniqueImages(entries); len(got) != 1 || got["img1"].Type != "backglass" {
		t.Errorf("images: %+v", got)
	}
}

func TestInsertManufacturersDedupes(t *testing.T) {
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO manufacturers").
		WithArgs(8, "Bally", "Bally Manufacturing Co.").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: sampleEntries()})
	if _, err := inTx(db, db.insertManufacturers); err != nil {
		t.Fatal(err)
	}
	done(t, mock)
}

func TestInsertCountsSkippedRows(t *testing.T) {
	mock := newMock(t)
	mock.ExpectBegin()
	// ON CONFLICT DO NOTHING hit an existing row: zero rows affected
	mock.ExpectExec("INSERT INTO manufacturers").
		WithArgs(8, "Bally", "Bally Manufacturing Co.").
		WillReturnResult(pgxmock.NewResult("INSERT", 0))
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: sampleEntries()})
	tallies, err := inTx(db, db.insertManufacturers)
	if err != nil {
		t.Fatal(err)
	}
	done(t, mock)

	if len(tallies) != 1 || tallies[0].table != tableManufacturers || tallies[0].inserted != 0 || tallies[0].skipped != 1 {
		t.Fatalf("got %+v, want manufacturers inserted=0 skipped=1", tallies)
	}
}

func TestInsertPeopleAndFeatures(t *testing.T) {
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO people").WithArgs(3, "Jane").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO features").WithArgs(1, "Ramp", "playfield").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: sampleEntries()})
	if _, err := inTx(db, db.insertPeople); err != nil {
		t.Fatal(err)
	}
	if _, err := inTx(db, db.insertFeatures); err != nil {
		t.Fatal(err)
	}
	done(t, mock)
}

func TestInsertImagesSkipsEmptyVariants(t *testing.T) {
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO images").WithArgs("img1", (*string)(nil), true, "backglass").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// medium has no URL, so only large and small are inserted (in that order of the variants list)
	mock.ExpectExec("INSERT INTO image_variants").WithArgs("img1", "large", "http://x/l.jpg", 10, 20).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO image_variants").WithArgs("img1", "small", "http://x/s.jpg", 1, 2).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: sampleEntries()})
	if _, err := inTx(db, db.insertImages); err != nil {
		t.Fatal(err)
	}
	done(t, mock)
}

func TestInsertOPDBInsertsRelations(t *testing.T) {
	mock := newMock(t)
	entries := sampleEntries()[:1]

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO opdb ").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO opdb_people").WithArgs("A", 3, "Design", 0).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO opdb_features").WithArgs("A", 1).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO opdb_images").WithArgs("A", "img1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: entries})
	if _, err := inTx(db, db.insertOPDB); err != nil {
		t.Fatal(err)
	}
	done(t, mock)
}

func TestInsertRollsBackOnError(t *testing.T) {
	mock := newMock(t)
	boom := errors.New("boom")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO manufacturers").WithArgs(8, "Bally", "Bally Manufacturing Co.").WillReturnError(boom)
	mock.ExpectRollback()
	mock.ExpectRollback().WillReturnError(pgx.ErrTxClosed) // pgx also rolls back in a defer

	db := NewWithDB(mock, &opdbv2.Export{Entries: sampleEntries()})
	_, err := inTx(db, db.insertManufacturers)
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
	if !strings.Contains(err.Error(), "manufacturer 8") {
		t.Fatalf("error should name the manufacturer: %v", err)
	}
	done(t, mock)
}

func entryWithManufacturer() opdbv2.OPDB {
	return opdbv2.OPDB{OPDBID: "A", Manufacturer: &opdbv2.Manufacturer{ManufacturerID: 8, Name: "Bally", FullName: "Bally Manufacturing Co."}}
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func TestLoadToDBOrder(t *testing.T) {
	mock := newMock(t) // expectations are ordered by default
	ok := pgxmock.NewResult("INSERT", 1)

	// One transaction for the whole load. Parents come first so foreign keys
	// resolve; the entry only has a manufacturer, so people, features and
	// images issue no statements.
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO manufacturers").WithArgs(8, "Bally", "Bally Manufacturing Co.").WillReturnResult(ok)
	mock.ExpectExec("INSERT INTO opdb ").WithArgs(anyArgs(28)...).WillReturnResult(ok)
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{entryWithManufacturer()}})
	if err := db.LoadToDB(context.Background(), LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	done(t, mock)
}

func TestLoadToDBTruncatesInSameTransaction(t *testing.T) {
	mock := newMock(t)
	ok := pgxmock.NewResult("INSERT", 1)

	mock.ExpectBegin()
	mock.ExpectExec(`TRUNCATE TABLE opdb_people, people, opdb_images, images, image_variants, features, opdb_features, opdb, manufacturers RESTART IDENTITY`).
		WillReturnResult(pgxmock.NewResult("TRUNCATE", 0))
	mock.ExpectExec("INSERT INTO manufacturers").WithArgs(8, "Bally", "Bally Manufacturing Co.").WillReturnResult(ok)
	mock.ExpectExec("INSERT INTO opdb ").WithArgs(anyArgs(28)...).WillReturnResult(ok)
	expectCommit(mock)

	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{entryWithManufacturer()}})
	if err := db.LoadToDB(context.Background(), LoadOptions{Truncate: true}); err != nil {
		t.Fatal(err)
	}
	done(t, mock)
}

// A failure in a late step must roll back the earlier steps and the truncate:
// there is a single Begin and no Commit.
func TestLoadToDBRollsBackEverythingOnError(t *testing.T) {
	mock := newMock(t)
	boom := errors.New("boom")

	mock.ExpectBegin()
	mock.ExpectExec("TRUNCATE TABLE").WillReturnResult(pgxmock.NewResult("TRUNCATE", 0))
	mock.ExpectExec("INSERT INTO manufacturers").WithArgs(8, "Bally", "Bally Manufacturing Co.").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO opdb ").WithArgs(anyArgs(28)...).WillReturnError(boom)
	mock.ExpectRollback()
	mock.ExpectRollback().WillReturnError(pgx.ErrTxClosed) // pgx also rolls back in a defer

	var buf bytes.Buffer
	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{entryWithManufacturer()}})
	db.log = slog.New(slog.NewTextHandler(&buf, nil))
	err := db.LoadToDB(context.Background(), LoadOptions{Truncate: true})
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
	done(t, mock)

	if !strings.Contains(buf.String(), "transaction rolled back") || strings.Contains(buf.String(), "import committed") {
		t.Errorf("expected a rollback message and no commit message:\n%s", buf.String())
	}
}

func TestLoadToDBLogsCountsAndCommit(t *testing.T) {
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO manufacturers").WithArgs(8, "Bally", "Bally Manufacturing Co.").
		WillReturnResult(pgxmock.NewResult("INSERT", 0)) // already in the database
	mock.ExpectExec("INSERT INTO opdb ").WithArgs(anyArgs(28)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)

	var buf bytes.Buffer
	db := NewWithDB(mock, &opdbv2.Export{Entries: []opdbv2.OPDB{entryWithManufacturer()}})
	db.log = slog.New(slog.NewTextHandler(&buf, nil))
	if err := db.LoadToDB(context.Background(), LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	done(t, mock)

	for _, want := range []string{"table=manufacturers inserted=0 skipped=1", "table=opdb inserted=1 skipped=0", "import committed"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log %q does not contain %q", buf.String(), want)
		}
	}
}

func TestWithLoggerLogsSQLAtDebug(t *testing.T) {
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO manufacturers").WithArgs(8, "Bally", "Bally Manufacturing Co.").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	expectCommit(mock)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	db := NewWithDB(mock, &opdbv2.Export{Entries: sampleEntries()}).WithLogger(log)
	if _, err := inTx(db, db.insertManufacturers); err != nil {
		t.Fatal(err)
	}
	done(t, mock)

	out := buf.String()
	if !strings.Contains(out, "INSERT INTO manufacturers") || !strings.Contains(out, "Bally") {
		t.Fatalf("expected SQL and args in debug log, got:\n%s", out)
	}
}

func TestWithLoggerTwiceReplacesLogger(t *testing.T) {
	mock := newMock(t)
	mock.ExpectExec("SELECT 1").WillReturnResult(pgxmock.NewResult("SELECT", 1))

	var first, second bytes.Buffer
	newLog := func(b *bytes.Buffer) *slog.Logger {
		return slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	db := NewWithDB(mock, &opdbv2.Export{}).WithLogger(newLog(&first)).WithLogger(newLog(&second))

	if _, err := db.conn.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	done(t, mock)

	if first.Len() != 0 {
		t.Errorf("replaced logger should get nothing, got:\n%s", first.String())
	}
	if got := strings.Count(second.String(), "SELECT 1"); got != 1 {
		t.Errorf("statement should be logged exactly once, got %d:\n%s", got, second.String())
	}
}

// closeSpy records the state of the context Close was called with.
type closeSpy struct {
	DB
	called      bool
	errAtCall   error
	hasDeadline bool
	err         error
}

func (c *closeSpy) Close(ctx context.Context) error {
	c.called = true
	c.errAtCall = ctx.Err()
	_, c.hasDeadline = ctx.Deadline()
	return c.err
}

func TestCloseGracefully(t *testing.T) {
	t.Run("close context is live and bounded when the parent is cancelled", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()

		spy := &closeSpy{}
		NewWithDB(spy, &opdbv2.Export{}).CloseGracefully(cancelled)

		if !spy.called {
			t.Fatal("Close was not called")
		}
		if spy.errAtCall != nil {
			t.Errorf("Close ctx already done: %v", spy.errAtCall)
		}
		if !spy.hasDeadline {
			t.Error("Close ctx has no deadline")
		}
	})

	t.Run("logs close error instead of returning it", func(t *testing.T) {
		var buf bytes.Buffer
		log := slog.New(slog.NewTextHandler(&buf, nil))
		spy := &closeSpy{err: errors.New("boom")}
		NewWithDB(spy, &opdbv2.Export{}).WithLogger(log).CloseGracefully(context.Background())

		if !strings.Contains(buf.String(), "boom") {
			t.Errorf("close error not logged: %q", buf.String())
		}
	})
}
