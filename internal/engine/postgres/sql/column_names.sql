-- Columns of one table in declared order ($1 schema, $2 table).
select a.attname::text
from pg_attribute a
join pg_class c on c.oid = a.attrelid
join pg_namespace n on n.oid = c.relnamespace
where n.nspname = $1
  and c.relname = $2
  and c.relkind in ('r', 'p', 'v', 'm', 'f')
  and a.attnum > 0
  and not a.attisdropped
order by a.attnum
