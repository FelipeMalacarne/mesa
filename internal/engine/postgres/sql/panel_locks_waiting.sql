select count(*)::int from pg_locks where not granted
