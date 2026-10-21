select count(*)::int
from pg_stat_activity
where xact_start < now() - interval '5 minutes'
  and pid <> pg_backend_pid()
