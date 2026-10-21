-- Databases on the server, templates excluded.
select d.datname::text,
       pg_get_userbyid(d.datdba)::text as owner,
       pg_encoding_to_char(d.encoding)::text as encoding,
       case when has_database_privilege(d.datname, 'CONNECT') then pg_database_size(d.datname) else 0 end as size_bytes
from pg_database d
where not d.datistemplate
order by d.datname
