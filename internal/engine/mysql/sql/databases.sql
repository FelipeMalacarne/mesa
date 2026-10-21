SELECT s.schema_name,
       s.default_character_set_name,
       COALESCE((SELECT SUM(t.data_length + t.index_length)
                 FROM information_schema.tables t
                 WHERE t.table_schema = s.schema_name), 0)
FROM information_schema.schemata s
WHERE s.schema_name NOT IN ('mysql', 'sys', 'information_schema', 'performance_schema')
   OR s.schema_name = DATABASE()
ORDER BY s.schema_name
