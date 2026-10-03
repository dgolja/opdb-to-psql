# opdb-to-psql

Import the [Open Pinball Database (OPDB)](https://opdb.org) JSON export into a normalized
PostgreSQL schema, and export it back to the original OPDB JSON format.

It supports the **Full dataset (V2)** export format, as described in the Matchplay
[data exports](https://docs.matchplay.events/data-exports) documentation.

The schema and importer target plain PostgreSQL. [Supabase](https://supabase.com) is used for
convenience (local Postgres via Docker, migrations, and a PostgREST layer for the export
view/RPCs), but it isn't required. The migrations in `supabase/migrations/` are plain SQL that
can be applied to any PostgreSQL database, and the Go importer/exporter only need a `DB_URL`.

Although, as mentioned above, Supabase is not required, it is a good fit for your projects. It
has also a [free plan](https://supabase.com/pricing), and its Data API exposes your tables over a REST
API (PostgREST), so you can consume the OPDB data from an app without writing your own backend.

## Why

OPDB publishes its full dataset as a single large JSON file: a flat list of machine **entries**,
each with nested manufacturer, people, image, and feature data. This repo turns that file into a
relational schema, with tables for `manufacturers`, `people`, `images`/`image_variants`,
`features`, and the core `opdb` table, linked through join tables (`opdb_people`, `opdb_images`,
`opdb_features`). The data can then be queried, indexed, and served like normal application data,
and still be reconstructed into the original JSON format when needed.

Importing the data into your own database is also what Matchplay recommends. Its
[API documentation](https://docs.matchplay.events/api) says OPDB data should be fetched from the
CDN (the data exports) and not through the API, and that your architecture should include its own
storage rather than treating the Matchplay API as a backend. Calling the API for this data can
run into rate limits and IP bans. Keeping a local copy avoids those problems.

## Repository layout

- **[`supabase/`](supabase/)** — Supabase project: SQL migrations defining the schema,  plus a view/RPC layer (`opdb_export`, `opdb_entries()`, `opdb_entry()`) that reassembles the normalized tables back into the original OPDB JSON structure, which can be exposed over PostgREST.
- **[`tools/opdb-importer/`](tools/opdb-importer/)** — a Go CLI (`opdb-importer`) with two subcommands:
  - `import` — reads an OPDB JSON export and loads it into the Postgres tables.
  - `export` — reads the tables back out and writes an OPDB-shaped JSON file (useful for
    verifying round-trip correctness or taking a backup).
- **[`tools/download-opdb-v2.sh`](tools/download-opdb-v2.sh)** — downloads the latest OPDB v2 JSON export, using the server's
  ETag to skip re-downloading unchanged data.
- **[`tools/package.json`](tools/package.json)** — JS dev dependencies (`ajv-cli`, `ajv-formats`) used to validate a
  downloaded export against the JSON schema.
- **[`opdb-v2-schema.json`](opdb-v2-schema.json)** — JSON Schema describing the shape of an OPDB v2 export, used for
  validation. This is an unofficial schema written for this project. It was not published by
  Matchplay or OPDB, and may not match every export exactly.

## Installation

Install the `opdb-importer` CLI with Go (requires a recent Go toolchain, see `go.mod`):

```bash
go install github.com/dgolja/opdb-to-psql/tools/opdb-importer@latest
```

To build from a clone of this repository instead:

```bash
cd tools/opdb-importer
go install .   # or: go build -o opdb-importer .
```

## Usage

### 1. Get the data

Matchplay publishes the OPDB v2 export at
[docs.matchplay.events/data-exports](https://docs.matchplay.events/data-exports), along with
format notes. Download it from there, or use the helper script, which fetches the same file and
skips the download if it hasn't changed:

```bash
cd tools
./download-opdb-v2.sh ../data  # downloads data/opdb-v2.<timestamp>.<etag>.json
```

Validate it against the schema (`opdb-v2-schema.json`, an unofficial schema, see
[Repository layout](#repository-layout)) before importing:

```bash
npm install  # first time only: installs ajv-cli, which runs the validation
npm run validate -- ../data/opdb-v2.<timestamp>.<etag>.json
```

> [!NOTE]
> This step is worth doing every time. A malformed or unexpected file makes the import fail (and roll back, see [The import is atomic](#the-import-is-atomic)), and only after it has done the work up to the bad row. Validation catches wrong types, missing required fields and format changes in the export up front, before anything is written.

### 2. Apply the schema

Using Supabase locally:

```bash
supabase start
supabase migration up
```

Or, without Supabase, apply the SQL files in `supabase/migrations/` directly, in order, to any
PostgreSQL database:

```bash
for f in supabase/migrations/*.sql; do psql "$DB_URL" -f "$f"; done
```

### 3. Import into Postgres

```bash
opdb-importer import -f data/opdb-v2.<timestamp>.<etag>.json -d postgresql://postgres:postgres@127.0.0.1:54322/postgres
```

`--db-url`/`-d` can also be set via the `DB_URL` environment variable.

#### The import is atomic

The whole import runs in a single transaction. That covers the truncate (if requested) and the inserts for manufacturers, people, features, images, and the `opdb` entries with their links.

If the file contains bad data, or the import is interrupted or fails for any other reason, the transaction is rolled back and the database is left exactly as it was before. Nothing is partly imported, and with `--truncate` the existing data is not lost when the import fails. Fix the data and run the import again.

> [!IMPORTANT]
> With `--truncate` the transaction holds an exclusive lock on the OPDB tables until it commits, so queries on them wait for the import to finish. [Validating the file against the schema](#1-get-the-data) first still saves you from waiting for a failure late in the file.

#### Re-importing and `--truncate`

Every insert uses `ON CONFLICT ... DO NOTHING`, so the import is safe to run more than once, but it never updates existing rows. If a row with the same key is already in the database it is left as is. Re-importing a newer OPDB export will add the new entries, but changed names, URLs or `updatedAt` values on entries that already exist will not be picked up.

To fully replace the data with the contents of the file, add `--truncate`:

```bash
opdb-importer import -f data/opdb-v2.<timestamp>.<etag>.json --truncate
```

This empties all OPDB tables with `TRUNCATE ... RESTART IDENTITY` before loading. It is off by default because it deletes all existing data in those tables. The truncate is part of the import transaction, so if the import fails the old data is still there.


> [!NOTE]
>  A separate `sync` mode that applies changes from a newer export file to existing tables is planned.

#### Reading the import output

After each step the importer logs one line per table, with the number of rows inserted and
skipped:

```
level=INFO msg="step done" table=manufacturers inserted=0 skipped=120
level=INFO msg="step done" table=opdb inserted=35 skipped=19965
```

- **inserted** is the number of rows that were new and were written to the table.
- **skipped** is the number of rows that were not written because a row with the same key already exists (the `ON CONFLICT ... DO NOTHING` case above). A skip is not an error.

A skipped row means only that the key exists, not that its contents match the file. A changed entry from a newer export is also counted as skipped and keeps its old values.

On a first import into empty tables `skipped` is 0 everywhere. When re-importing the same file, `inserted` is 0 and `skipped` equals the number of rows. When re-importing a newer export, `inserted` shows how many new rows were added. If `skipped` is unexpectedly high, the tables already contained data; use `--truncate` to replace it.

The counts are logged as each step finishes, but they only become final when the whole import commits. A successful run ends with `import committed`. If a later step fails, the importer logs `import failed, transaction rolled back` and none of the rows counted above were saved.

### 4. Export back to JSON (verification / backup)

```bash
opdb-importer export -f export.json -d postgresql://postgres:postgres@127.0.0.1:54322/postgres
```

The exported file can be diffed against the original (ignoring timestamps) with tools like
[`jd`](https://github.com/josephburnett/jd). Example:

```bash
jd -opts='[{"@":[],"^":[{"setkeys":["opdbId"]}]},{"@":[{},"updatedAt"],"^":["DIFF_OFF"]}]' FILE1 FILE2
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup and how to run the unit and
integration tests.

## Backwards compatibility

Until the code is stabilised I may add breaking changes, but I will avoid them where possible.

The OPDB JSON export is produced by its own author, not by this project. If the export format changes in a breaking way, this project will have to be adjusted to follow it, which may in turn require breaking changes here.

## License

opdb-to-psql is released under the MIT license.
