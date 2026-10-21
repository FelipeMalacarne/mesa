-- Functions and procedures, extension members excluded ($1 schema filter).
select n.nspname::text, p.proname::text,
       pg_get_function_identity_arguments(p.oid) as signature,
       coalesce(pg_get_function_result(p.oid), '') as returns,
       l.lanname::text as language
from pg_proc p
join pg_namespace n on n.oid = p.pronamespace
join pg_language l on l.oid = p.prolang
where p.prokind in ('f', 'p')
  and not exists (select 1 from pg_depend d
                  where d.classid = 'pg_proc'::regclass and d.objid = p.oid and d.deptype = 'e')
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
order by n.nspname, p.proname, signature
