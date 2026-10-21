select coalesce(application_name, ''),
       coalesce(host(client_addr), ''),
       coalesce(state, ''),
       coalesce(sync_state, ''),
       coalesce(sent_lsn::text, ''),
       coalesce(replay_lag::text, '')
from pg_stat_replication
order by application_name
