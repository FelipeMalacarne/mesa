SELECT tc.table_name, tc.constraint_name, tc.constraint_type,
       k.column_name, COALESCE(k.referenced_table_schema, ''), COALESCE(k.referenced_table_name, ''),
       COALESCE(k.referenced_column_name, ''),
       COALESCE(r.delete_rule, ''), COALESCE(r.update_rule, '')
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage k
  ON k.constraint_schema = tc.constraint_schema
 AND k.constraint_name = tc.constraint_name
 AND k.table_name = tc.table_name
LEFT JOIN information_schema.referential_constraints r
  ON r.constraint_schema = tc.constraint_schema
 AND r.constraint_name = tc.constraint_name
 AND r.table_name = tc.table_name
WHERE tc.table_schema = ? AND (? IS NULL OR tc.table_name = ?)
  AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE', 'FOREIGN KEY')
ORDER BY tc.table_name, tc.constraint_name, k.ordinal_position
