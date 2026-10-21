-- Client backends other than this one, with whether each blocks another.
with act as (
  select *
  from pg_stat_activity
  where backend_type = 'client backend' and pid <> pg_backend_pid()
),
blockers as (
  select distinct unnest(pg_blocking_pids(pid)) as pid from act
)
select a.pid,
       coalesce(a.usename::text, ''),
       coalesce(a.datname::text, ''),
       coalesce(a.application_name, ''),
       coalesce(host(a.client_addr), ''),
       coalesce(a.state, ''),
       coalesce(a.wait_event_type || ': ' || a.wait_event, ''),
       coalesce(a.query, ''),
       a.backend_start,
       a.query_start,
       a.xact_start,
       now(),
       exists (select 1 from blockers b where b.pid = a.pid) as blocking
from act a
order by a.backend_start
