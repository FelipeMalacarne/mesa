SELECT tc.table_name, cc.constraint_name, cc.check_clause
FROM information_schema.table_constraints tc
JOIN information_schema.check_constraints cc
  ON cc.constraint_schema = tc.constraint_schema
 AND cc.constraint_name = tc.constraint_name
WHERE tc.table_schema = ? AND (? IS NULL OR tc.table_name = ?)
  AND tc.constraint_type = 'CHECK'
ORDER BY tc.table_name, cc.constraint_name
