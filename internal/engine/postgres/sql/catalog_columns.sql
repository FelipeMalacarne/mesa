-- Columns of tables and views ($1 schema filter, $2 relation filter).
select n.nspname::text, c.relname::text, a.attname::text,
       format_type(a.atttypid, a.atttypmod) as type,
       not a.attnotnull as nullable,
       case when a.attgenerated = '' then pg_get_expr(d.adbin, d.adrelid) end as default_expr,
       a.attidentity::text as identity,
       coalesce(col_description(c.oid, a.attnum), '') as comment
from pg_attribute a
join pg_class c on c.oid = a.attrelid
join pg_namespace n on n.oid = c.relnamespace
left join pg_attrdef d on d.adrelid = a.attrelid and d.adnum = a.attnum
where a.attnum > 0
  and not a.attisdropped
  and c.relkind in ('r', 'p', 'v', 'm')
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
  and ($2::text is null or c.relname = $2)
order by n.nspname, c.relname, a.attnum
