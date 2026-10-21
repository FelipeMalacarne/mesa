-- Server identity for the Monitor header.
select version(),
       current_setting('server_version_num')::int,
       pg_postmaster_start_time(),
       current_database()::text,
       current_user::text,
       pg_is_in_recovery()
