-- General counters of the current database and server.
with db as (
  select xact_commit, xact_rollback, blks_hit, blks_read
  from pg_stat_database
  where datname = current_database()
),
act as (
  select state, xact_start
  from pg_stat_activity
  where backend_type = 'client backend' and pid <> pg_backend_pid()
)
select coalesce((select xact_commit from db), 0)::bigint,
       coalesce((select xact_rollback from db), 0)::bigint,
       coalesce((select blks_hit from db), 0)::bigint,
       coalesce((select blks_read from db), 0)::bigint,
       (select count(*) from act where state = 'active')::int,
       (select count(*) from act where state = 'idle')::int,
       (select count(*) from act where state in ('idle in transaction', 'idle in transaction (aborted)'))::int,
       current_setting('max_connections')::int,
       pg_database_size(current_database())::bigint,
       (select count(*) from pg_locks where not granted)::int,
       coalesce((select max(extract(epoch from now() - xact_start)) from act where xact_start is not null), 0)::float8
