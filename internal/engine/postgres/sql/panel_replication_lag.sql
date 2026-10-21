select count(*)::int,
       coalesce(max(extract(epoch from replay_lag)), 0)::float8
from pg_stat_replication
