-- Secondary indexes with stats; indexes backing constraints are left out
-- because the constraints carry them ($1 schema, $2 table).
select n.nspname::text, c.relname::text, ic.relname::text as index_name,
       i.indisunique, am.amname::text,
       coalesce(pg_get_expr(i.indpred, i.indrelid), '') as predicate,
       coalesce((select array_agg(coalesce(a.attname::text, '') order by k.ord)
                 from unnest(i.indkey::int2[]) with ordinality as k(attnum, ord)
                 left join pg_attribute a on a.attrelid = i.indrelid and a.attnum = k.attnum
                 where k.ord <= i.indnkeyatts), '{}') as columns,
       coalesce((select array_agg(pg_get_indexdef(i.indexrelid, k.ord::int, true) order by k.ord)
                 from generate_series(1, i.indnkeyatts) as k(ord)), '{}') as keys,
       pg_relation_size(i.indexrelid) as size_bytes,
       coalesce(s.idx_scan, 0)::bigint as scans
from pg_index i
join pg_class ic on ic.oid = i.indexrelid
join pg_class c on c.oid = i.indrelid
join pg_namespace n on n.oid = c.relnamespace
join pg_am am on am.oid = ic.relam
left join pg_stat_user_indexes s on s.indexrelid = i.indexrelid
where not i.indisprimary
  and not exists (select 1 from pg_constraint con where con.conindid = i.indexrelid and con.contype in ('p', 'u', 'x'))
  and c.relkind in ('r', 'p', 'm')
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
  and ($2::text is null or c.relname = $2)
order by n.nspname, c.relname, ic.relname
