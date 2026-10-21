select exists (select 1 from pg_extension where extname = 'pg_stat_statements')
