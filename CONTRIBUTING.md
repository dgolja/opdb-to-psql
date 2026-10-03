# Contributing

Thanks for your interest in contributing to opdb-to-psql. Bug reports, fixes and improvements are
welcome. For an overview of the project and how to use it, see the [README](README.md).

## Development setup

You will need:

- [Go](https://go.dev/dl/) (see `tools/opdb-importer/go.mod` for the version)
- A PostgreSQL database. The easiest option is the local one from the
  [Supabase CLI](https://supabase.com/docs/guides/local-development), which needs Docker:

  ```bash
  supabase start
  supabase migration up
  ```

  Any PostgreSQL database works. See [Apply the schema](README.md#2-apply-the-schema).
- Node.js and npm, only if you want to validate OPDB exports (see
  [Get the data](README.md#1-get-the-data)).

The importer lives in `tools/opdb-importer`. Build it from the repository root with `make build`.
Run `make help` to list all available targets.

## Tests

Run these from the repository root.

### Unit tests

```bash
make test
```

These need no database. The `pgsql` package is tested against a mocked connection
([`pgxmock`](https://github.com/pashagolub/pgxmock)).

### Integration tests

The integration tests import a sample OPDB file into a real PostgreSQL database, export it again
and check that the result matches the input. They also cover row counts, re-importing,
`--truncate`, and that a failed import changes nothing (see
[The import is atomic](README.md#the-import-is-atomic)).

They are behind the `integration` build tag, so `make test` never runs them. Start the local
database first (`supabase start`), then run:

```bash
TEST_DB_URL=postgresql://postgres:postgres@127.0.0.1:54322/postgres make test-integration
```

- `TEST_DB_URL` is required. If it is not set, the tests are skipped.
- Existing data is not touched. Each test creates its own temporary schema, applies
  `supabase/migrations/` into it, and drops it when the test finishes. Any PostgreSQL database
  works, not only Supabase.
- The tests use `tools/opdb-importer/testdata/opdb-v2.sample.json` by default. To use another
  export, set `TEST_DATA_FILE` to its path (use an absolute path):

  ```bash
  TEST_DB_URL=... TEST_DATA_FILE=/path/to/opdb-v2.json make test-integration
  ```

- To keep the temporary schemas and their data after the run, for example to inspect a failing
  test, set `TEST_KEEP_SCHEMA=1` and run the Go test command directly with `-v` (from
  `tools/opdb-importer`: `go test -v -tags integration ./internal/pgsql`) to see the schema names
  in the output, as `make test-integration` does not pass `-v`. Connect to the
  database, run `SET search_path = it_<name>;`, and query the tables. The schemas are not removed
  automatically, so drop them yourself when you are done (`DROP SCHEMA it_<name> CASCADE;`).
- The full run takes about 15 seconds with the default sample, because rows are inserted one at a
  time.

## Submitting changes

1. Open an issue first for larger changes, so the approach can be agreed before you spend time on it.
2. Make sure `make fmt`, `make vet` and `make test` pass. Run `make test-integration` too if you
   changed the importer, exporter or the SQL migrations.
3. Open a pull request with a short description of what changed and why.
