# Supabase project via Pulumi (IaC)

IaC for creating and configuring a [Supabase](https://supabase.com) project with [Pulumi](https://www.pulumi.com). It sets up the project used by the [Data API guide](../../README.DataAPI.md) in the parent repo. Running `pulumi up` will:

1. **Create the Supabase project** (organization, region, instance size, database password).
2. **Disable "Automatically expose new tables"** on the `public` schema. The Supabase provider has no field for this dashboard toggle, so a small Pulumi dynamic resource revokes the default privileges (tables, functions, sequences) from `anon`, `authenticated` and `service_role` via the Management API. New tables are then **not** exposed through the Data API (PostgREST) until you explicitly `GRANT` access to them.
3. **Configure the Data API settings** (exposed schema `public`, extra search path `public,extensions`, `max_rows` 1000).

## Prerequisites

- [Pulumi CLI](https://www.pulumi.com/docs/install/)
- [Node.js](https://nodejs.org) and npm
- A Supabase account and a [personal access token](https://supabase.com/dashboard/account/tokens)

## Basic setup

```console
pulumi login --local
pulumi install
pulumi stack init dev   # or/and copy Pulumi.dev.example.yaml to Pulumi.dev.yaml
pulumi config set projectName PROJECT_NAME # default opdb-to-psql
# orgId is your organization's slug (it can be found in the Supabase Dashboard under Organization Settings > Organization Details > Organization slug)
pulumi config set orgId skrXXXXX
pulumi config set dbPassword --secret
# Supabase personal access token (used by the provider and by the SQL step).
# Alternatively export SUPABASE_ACCESS_TOKEN instead.
pulumi config set supabase:accessToken --secret
pulumi up
```

> [!NOTE]
> `pulumi login --local` keeps state in `~/.pulumi`. Secrets in it (the DB password and the access token) are encrypted with a passphrase; set `PULUMI_CONFIG_PASSPHRASE` to avoid the prompt. Back up the state and passphrase, or you lose track of the project.

After `pulumi up`, the project reference is available as a stack output:

```console
pulumi stack output projectRef
```

The API URL and keys are in the Supabase Dashboard under Project Settings > API.

### Optionally

By default the project **region** is `ap-southeast-2`. To set a different location:

```console
pulumi config set region us-east-1
```

The free plan only allows **nano**, which is the default when **instanceSize** is unset. Any other size needs a paid plan.

```console
pulumi config set instanceSize micro
```

> [!IMPORTANT]
> Do not set instanceSize to `nano` explicitly, due to a [known provider bug (#372)](https://github.com/supabase/terraform-provider-supabase/issues/372).

## Exposing tables after setup

Because auto-exposure is disabled, each table you want available through the Data API needs an explicit grant and row level security. Without RLS, granting to `anon` makes the table's data public. For our case described in [README.DataAPI.md](../../README.DataAPI.md) after the schema creation you would need to run:

```sql
-- Only signed-in users can fetch the whole dataset
GRANT EXECUTE ON FUNCTION public.opdb_entries() TO authenticated;

-- Anyone with the publishable key can fetch a single entry
GRANT EXECUTE ON FUNCTION public.opdb_entry(text, boolean, boolean) TO authenticated, anon;
```
