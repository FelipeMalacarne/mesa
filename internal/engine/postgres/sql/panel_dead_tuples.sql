select schemaname || '.' || relname,
       n_dead_tup::bigint,
       coalesce(round(100.0 * n_dead_tup / nullif(n_live_tup + n_dead_tup, 0), 1), 0)::float8,
       last_autovacuum
from pg_stat_user_tables
where n_dead_tup > 0
order by 3 desc, 2 desc
limit 5
