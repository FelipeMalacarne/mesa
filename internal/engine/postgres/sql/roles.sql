-- Roles with the groups they belong to; predefined pg_* roles excluded.
select r.rolname::text, r.rolsuper, r.rolcanlogin, r.rolcreatedb, r.rolcreaterole, r.rolinherit,
       r.rolconnlimit, r.rolvaliduntil,
       coalesce((select array_agg(g.rolname::text order by g.rolname)
                 from pg_auth_members m
                 join pg_roles g on g.oid = m.roleid
                 where m.member = r.oid), '{}') as member_of
from pg_roles r
where r.rolname not like 'pg\_%'
order by r.rolname
