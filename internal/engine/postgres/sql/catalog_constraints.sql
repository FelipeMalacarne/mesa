-- Primary keys, uniques, checks and foreign keys ($1 schema, $2 table).
select n.nspname::text, c.relname::text, con.conname::text, con.contype::text,
       coalesce((select array_agg(a.attname::text order by k.ord)
                 from unnest(con.conkey) with ordinality as k(attnum, ord)
                 join pg_attribute a on a.attrelid = con.conrelid and a.attnum = k.attnum), '{}') as columns,
       coalesce(rn.nspname::text, '') as ref_schema,
       coalesce(rc.relname::text, '') as ref_table,
       coalesce((select array_agg(a.attname::text order by k.ord)
                 from unnest(con.confkey) with ordinality as k(attnum, ord)
                 join pg_attribute a on a.attrelid = con.confrelid and a.attnum = k.attnum), '{}') as ref_columns,
       con.confdeltype::text, con.confupdtype::text,
       pg_get_constraintdef(con.oid) as definition
from pg_constraint con
join pg_class c on c.oid = con.conrelid
join pg_namespace n on n.oid = c.relnamespace
left join pg_class rc on rc.oid = con.confrelid
left join pg_namespace rn on rn.oid = rc.relnamespace
where con.contype in ('p', 'u', 'c', 'f')
  and c.relkind in ('r', 'p')
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
  and ($2::text is null or c.relname = $2)
order by n.nspname, c.relname, con.conname
