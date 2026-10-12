import * as pulumi from "@pulumi/pulumi";
import * as supabase from "@pulumi/supabase";

const config = new pulumi.Config();

const projectName  = config.require("projectName");
const region       = config.get("region") ?? "ap-southeast-2";
const orgId        = config.require("orgId");
const dbPassword   = config.requireSecret("dbPassword");
const instanceSize = config.get("instanceSize");

const project = new supabase.Project("main", {
    organizationId: orgId,
    name: projectName,
    databasePassword: dbPassword,
    instanceSize: instanceSize,
    region: region,
});

export const projectRef = project.id;

// The Supabase provider has no field for "Automatically expose new tables", so
// revoke the default grants on `public` (what the dashboard toggle does) by
// running SQL through the Management API. Exposure then becomes opt-in:
// each table/function needs an explicit GRANT to anon/authenticated/service_role.
// Same token the provider uses: `supabase:accessToken` config, or the
// SUPABASE_ACCESS_TOKEN env var as a fallback.
const accessToken = new pulumi.Config("supabase").getSecret("accessToken")
    ?? pulumi.secret(process.env.SUPABASE_ACCESS_TOKEN ?? (() => {
        throw new Error("Set supabase:accessToken config or SUPABASE_ACCESS_TOKEN env var");
    })());

const NO_AUTO_EXPOSE_SQL = `
alter default privileges for role postgres in schema public
  revoke select, insert, update, delete on tables from anon, authenticated, service_role;
alter default privileges for role postgres in schema public
  revoke execute on functions from anon, authenticated, service_role;
alter default privileges for role postgres in schema public
  revoke usage, select on sequences from anon, authenticated, service_role;
alter default privileges for role postgres in schema public
  revoke execute on functions from public;
`;

// A freshly created project can take a while before it accepts queries, so
// retry a few times with a growing delay before giving up.
async function runSql(ref: string, token: string, query: string, attempts = 5) {
    let lastError = "";
    for (let i = 1; i <= attempts; i++) {
        try {
            const res = await fetch(`https://api.supabase.com/v1/projects/${ref}/database/query`, {
                method: "POST",
                headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
                body: JSON.stringify({ query }),
            });
            if (res.ok) {
                return;
            }
            lastError = `${res.status} ${await res.text()}`;
        } catch (e) {
            lastError = String(e);
        }
        if (i < attempts) {
            await new Promise((resolve) => setTimeout(resolve, i * 5000));
        }
    }
    throw new Error(`Supabase SQL query failed after ${attempts} attempts: ${lastError}`);
}

// Only the SQL is applied here; changing it re-runs it. Delete is a no-op
// (re-enabling auto-exposure is left to the dashboard).
// The token is deliberately left out of `outs`; it remains in the resource
// inputs, where Pulumi stores it encrypted because it is a secret.
const noAutoExposeProvider: pulumi.dynamic.ResourceProvider = {
    async create(inputs) {
        await runSql(inputs.ref, inputs.token, inputs.sql);
        return { id: `${inputs.ref}-no-auto-expose`, outs: { ref: inputs.ref, sql: inputs.sql } };
    },
    // Only `ref` and `sql` count as changes: a rotated/expired token must not
    // trigger a re-run. (`update` still receives the new token when it does run.)
    async diff(_id, olds, news) {
        return { changes: olds.ref !== news.ref || olds.sql !== news.sql };
    },
    async update(_id, _olds, news) {
        await runSql(news.ref, news.token, news.sql);
        return { outs: { ref: news.ref, sql: news.sql } };
    },
};

class NoAutoExposeNewTables extends pulumi.dynamic.Resource {
    constructor(name: string, args: { ref: pulumi.Input<string>; token: pulumi.Input<string>; sql: string }) {
        super(noAutoExposeProvider, name, args);
    }
}

const noAutoExpose = new NoAutoExposeNewTables("noAutoExposeNewTables", {
    ref: project.id,
    token: accessToken,
    sql: NO_AUTO_EXPOSE_SQL,
});

new supabase.Settings("settingsResource", {
    projectRef: project.id,
    api: JSON.stringify({
        db_schema: "public",
        db_extra_search_path: "public,extensions",
        max_rows: 1000,
    }),
}, { dependsOn: [noAutoExpose] });
