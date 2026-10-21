SELECT routine_name, routine_type, COALESCE(dtd_identifier, ''),
       COALESCE((SELECT GROUP_CONCAT(CONCAT_WS(' ', p.parameter_mode, p.parameter_name, p.dtd_identifier)
                                     ORDER BY p.ordinal_position SEPARATOR ', ')
                 FROM information_schema.parameters p
                 WHERE p.specific_schema = r.routine_schema
                   AND p.specific_name = r.specific_name
                   AND p.ordinal_position > 0), '')
FROM information_schema.routines r
WHERE routine_schema = ?
ORDER BY routine_name
