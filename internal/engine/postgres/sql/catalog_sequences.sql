-- Standalone sequences: those owned by identity or serial columns are
-- part of the column ($1 schema filter).
select n.nspname::text, c.relname::text
from pg_class c
join pg_namespace n on n.oid = c.relnamespace
where c.relkind = 'S'
  and not exists (select 1 from pg_depend d
                  where d.classid = 'pg_class'::regclass and d.objid = c.oid
                    and d.refclassid = 'pg_class'::regclass and d.deptype in ('a', 'i'))
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
order by n.nspname, c.relname
