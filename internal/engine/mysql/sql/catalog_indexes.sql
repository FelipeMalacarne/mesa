SELECT table_name, index_name, non_unique = 0, index_type,
       COALESCE(column_name, ''), COALESCE(expression, ''), sub_part
FROM information_schema.statistics
WHERE table_schema = ? AND (? IS NULL OR table_name = ?)
ORDER BY table_name, index_name, seq_in_index
