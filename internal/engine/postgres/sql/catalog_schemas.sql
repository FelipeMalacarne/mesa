-- User schemas ($1 optional schema filter).
select current_database()::text, n.nspname::text
from pg_namespace n
where n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
order by n.nspname
