SELECT u.user, u.host,
       u.super_priv = 'Y',
       u.account_locked = 'N',
       u.create_priv = 'Y',
       u.create_user_priv = 'Y',
       u.max_user_connections,
       COALESCE((SELECT GROUP_CONCAT(CASE WHEN e.from_host = '%' THEN e.from_user ELSE CONCAT(e.from_user, '@', e.from_host) END
                                     ORDER BY e.from_user SEPARATOR '\n')
                 FROM mysql.role_edges e
                 WHERE e.to_user = u.user AND e.to_host = u.host), '')
FROM mysql.user u
WHERE u.user NOT LIKE 'mysql.%' AND u.user <> ''
ORDER BY u.user, u.host
