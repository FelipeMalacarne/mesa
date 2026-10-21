-- Tables with stats ($1 schema filter, $2 table filter; null = all).
select n.nspname::text, c.relname::text,
       greatest(c.reltuples, 0)::bigint as row_estimate,
       pg_total_relation_size(c.oid) as size_bytes,
       coalesce(obj_description(c.oid, 'pg_class'), '') as comment
from pg_class c
join pg_namespace n on n.oid = c.relnamespace
where c.relkind in ('r', 'p')
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
  and ($2::text is null or c.relname = $2)
order by n.nspname, c.relname
