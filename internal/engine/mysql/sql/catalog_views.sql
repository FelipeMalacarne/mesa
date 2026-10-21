SELECT table_name, COALESCE(view_definition, '')
FROM information_schema.views
WHERE table_schema = ?
ORDER BY table_name
