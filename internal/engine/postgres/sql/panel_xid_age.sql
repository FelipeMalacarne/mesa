select coalesce(max(age(datfrozenxid)), 0)::bigint from pg_database
