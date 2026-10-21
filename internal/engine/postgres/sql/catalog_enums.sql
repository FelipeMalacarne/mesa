-- Enum types with their labels in order ($1 schema filter).
select n.nspname::text, t.typname::text,
       array_agg(e.enumlabel::text order by e.enumsortorder) as labels
from pg_type t
join pg_namespace n on n.oid = t.typnamespace
join pg_enum e on e.enumtypid = t.oid
where t.typtype = 'e'
  and n.nspname <> 'information_schema'
  and n.nspname not like 'pg\_%'
  and ($1::text is null or n.nspname = $1)
group by n.nspname, t.typname
order by n.nspname, t.typname
