-- Top statements by total time in this database (PostgreSQL 13+ column names).
select left(regexp_replace(query, '\s+', ' ', 'g'), 120),
       calls::bigint,
       round(mean_exec_time::numeric, 2)::float8,
       round((100 * total_exec_time / nullif(sum(total_exec_time) over (), 0))::numeric, 1)::float8
from pg_stat_statements
where dbid = (select oid from pg_database where datname = current_database())
order by total_exec_time desc
limit 10
