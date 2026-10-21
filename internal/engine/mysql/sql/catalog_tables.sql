SELECT table_name, table_type, COALESCE(table_rows, 0),
       COALESCE(data_length, 0) + COALESCE(index_length, 0), COALESCE(table_comment, '')
FROM information_schema.tables
WHERE table_schema = ? AND (? IS NULL OR table_name = ?)
ORDER BY table_name
