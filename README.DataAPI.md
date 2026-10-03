# Exposing OPDB data via the Supabase Data API

This guide shows how to expose the imported OPDB data through the [Supabase Data API](https://supabase.com/docs/guides/api) (PostgREST) using the functions created by [`supabase/migrations/20261003055315_opdb_func.sql`](supabase/migrations/20261003055315_opdb_func.sql).

> [!NOTE]
> **This is just an example.** It shows one way of exposing the data. Adapt the permissions to your own needs (see [Other variations](#other-variations)).

## What the example exposes

| Function | Equivalent Matchplay endpoint | Who can call it in this example |
| --- | --- | --- |
| `opdb_entry(opdbid, includepeople, includeimages)` | `GET /api/opdb/entry/{opdbId}` | Everyone with the publishable key (`anon` and `authenticated`) |
| `opdb_entries()` | the full export | Only signed-in users (`authenticated`) |

`opdb_entry` returns JSON formatted the same way as the Matchplay OPDB API, which is documented at [matchplay](https://docs.matchplay.events/opdb-and-pintips-api). Clients written against that API's `GET /api/opdb/entry/{opdbId}` response should be able to read the result.

### Why is `opdb_entries` restricted?

On purpose, `opdb_entries` is **not** open to the public. It returns the whole dataset, so it can generate a lot of traffic, and a bigger load may incur charges from Supabase. Only authenticated users can run it in this example.

### No TTL / caching on the data

> [!WARNING]
> PostgREST (and therefore the Supabase Data API) has no way to set a TTL on the data it serves. Every request is executed against the database, and the responses are not cached for you. The OPDB data rarely changes, so most of that traffic returns the same result again and again. Be aware of this because of Supabase pricing, which is driven mainly by egress and compute usage.

## Setting up with the Supabase CLI

Install the [Supabase CLI](https://supabase.com/docs/guides/local-development/cli/getting-started) first. The local stack also needs Docker.

### Local environment

From the root of this repository (it already contains `supabase/config.toml` and the migrations):

```console
# start the local Supabase stack (Postgres, PostgREST, Studio, ...)
supabase start

# apply the migrations in supabase/migrations/ to the local database
supabase migration up
```

### Link to a remote project

Create a project in the [Supabase dashboard](https://supabase.com/dashboard), then log in and link it. The project ref is the `<PROJECT_ID>` part of `https://<PROJECT_ID>.supabase.co`.

```console
supabase login
supabase link --project-ref <PROJECT_ID>
```

### Create the database on the remote project

Push the local migrations to the linked remote project:

```console
# preview what would be applied
supabase db push --dry-run

# apply the migrations to the remote database
supabase db push
```

This creates the OPDB schema and the `opdb_entry` / `opdb_entries` functions on the remote project. It does not import the OPDB data itself, use the importer for that.

## Disable automatic exposure of new tables

When you create a remote project, untick **Automatically expose new tables** on the project creation screen, before you push the migrations. When it is on, new objects in the exposed schema (`public`) are granted to `anon` and `authenticated` as soon as they are created (tables get `SELECT`/`INSERT`/`UPDATE`/`DELETE`, functions get `EXECUTE`), so you would have to remember to lock down each one. With it off, nothing is exposed until you explicitly grant access, which is the approach this guide takes (see [Permissions](#permissions)).

Supabase is making this opt-in behaviour the default:

- 2026-04-28: the opt-out checkbox is available at project creation.
- 2026-05-30: it becomes the default for new projects.
- 2026-10-30: it is enforced for all projects, existing ones included.

Until your project is switched over, the explicit `REVOKE` / `GRANT` statements in [Permissions](#permissions) are what keep the functions locked down, whatever the setting is.

More information: [supabase/discussions#45329](https://github.com/orgs/supabase/discussions/45329) and the [changelog entry](https://supabase.com/changelog/45329-breaking-change-tables-not-exposed-to-data-and-graphql-api-automatically).

## Permissions

First revoke all existing permissions so we know exactly what is granted. You can use the Supabase UI or the CLI:

```sql
REVOKE EXECUTE ON FUNCTION public.opdb_entries() FROM authenticated, anon, PUBLIC;
REVOKE EXECUTE ON FUNCTION public.opdb_entry(text, boolean, boolean) FROM authenticated, anon, PUBLIC;
```

Then grant permissions granularly to each function:

```sql
-- Only signed-in users can fetch the whole dataset
GRANT EXECUTE ON FUNCTION public.opdb_entries() TO authenticated;

-- Anyone with the publishable key can fetch a single entry
GRANT EXECUTE ON FUNCTION public.opdb_entry(text, boolean, boolean) TO authenticated, anon;
```

## Usage

As long as you have the project's Supabase **publishable key** (`sb_publishable_...`), this works.
You find it in the dashboard under *Project Settings → API Keys*.

Fetch a single entry:

```console
curl -H 'apikey: sb_publishable_XXXXX' \
  'https://<PROJECT_ID>.supabase.co/rest/v1/rpc/opdb_entry?opdbid=GrqZX-MD15w'
```

Include people and images as well:

```console
curl -H 'apikey: sb_publishable_XXXXX' \
  'https://<PROJECT_ID>.supabase.co/rest/v1/rpc/opdb_entry?opdbid=GrqZX-MD15w&includepeople=true&includeimages=true'
```

Fetch the whole dataset (requires a signed-in user's access token, the publishable key alone is not enough):

```console
curl -H 'apikey: sb_publishable_XXXXX' \
  -H 'Authorization: Bearer <USER_ACCESS_TOKEN>' \
  'https://<PROJECT_ID>.supabase.co/rest/v1/rpc/opdb_entries'
```

## Security advisor warning

The Supabase security advisor will show two warnings for this example:

- `0028_anon_security_definer_function_executable` for `public.opdb_entry(...)`: *"This `SECURITY DEFINER` function is callable without signing in."*
- `0029_authenticated_security_definer_function_executable` for `public.opdb_entry(...)` and `public.opdb_entries()`: *"This `SECURITY DEFINER` function is callable by signed-in users."*

These are **expected for this example** and can be ignored (or suppressed): letting these roles call the functions is the whole point. The functions are `SECURITY DEFINER` so callers can read the OPDB data without having direct access to the underlying tables. They set `search_path = ''` to stay safe. If you revoke access from a role (see [Other variations](#other-variations)), the matching warning goes away.

## Other variations

Everyone who has the publishable key can access `opdb_entry` in this example. If that is not what you want, for example if only authenticated users should have access, or you want some other variation, change the grants above (e.g. drop `anon` from the `opdb_entry` grant) and read the Supabase documentation: [Supabase Data API](https://supabase.com/docs/guides/api).
