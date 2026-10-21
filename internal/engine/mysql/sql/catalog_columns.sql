SELECT table_name, column_name, column_type, data_type, is_nullable = 'YES',
       column_default, extra, column_comment
FROM information_schema.columns
WHERE table_schema = ? AND (? IS NULL OR table_name = ?)
ORDER BY table_name, ordinal_position
