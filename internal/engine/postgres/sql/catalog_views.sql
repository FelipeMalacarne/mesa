-- Views and materialized views ($1 schema filter).
select n.nspname::text, c.relname::text, c.relkind = 'm' as materialized,
       pg_get_viewdef(c.oid, true) as definition
from pg_class c
join pg_namespace n on n.oid = c.relnamespace
where c.relkind in ('v', 'm')
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
order by n.nspname, c.relname
