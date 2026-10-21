select coalesce(blks_hit, 0)::bigint, coalesce(blks_read, 0)::bigint
from pg_stat_database
where datname = current_database()
