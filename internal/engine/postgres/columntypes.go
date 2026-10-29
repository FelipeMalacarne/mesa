package postgres

import "github.com/felipemalacarne/mesa/internal/engine"

// columnTypes are the types the designer offers for PostgreSQL, in the short
// spellings the catalog reports (see shortType).
var columnTypes = []engine.ColumnType{
	{Name: "bigint", Group: "Numeric", Spelling: "bigint", Integer: true},
	{Name: "integer", Group: "Numeric", Spelling: "integer", Integer: true},
	{Name: "smallint", Group: "Numeric", Spelling: "smallint", Integer: true},
	{Name: "numeric", Group: "Numeric", Args: "precision", Spelling: "numeric(12,2)"},
	{Name: "real", Group: "Numeric", Spelling: "real"},
	{Name: "float8", Group: "Numeric", Spelling: "float8"},
	{Name: "text", Group: "Text", Spelling: "text"},
	{Name: "varchar", Group: "Text", Args: "length", Spelling: "varchar(255)"},
	{Name: "char", Group: "Text", Args: "length", Spelling: "char(2)"},
	{Name: "citext", Group: "Text", Spelling: "citext"},
	{Name: "boolean", Group: "Boolean", Spelling: "boolean"},
	{Name: "timestamptz", Group: "Date & time", Spelling: "timestamptz"},
	{Name: "timestamp", Group: "Date & time", Spelling: "timestamp"},
	{Name: "date", Group: "Date & time", Spelling: "date"},
	{Name: "time", Group: "Date & time", Spelling: "time"},
	{Name: "interval", Group: "Date & time", Spelling: "interval"},
	{Name: "uuid", Group: "Identifiers", Spelling: "uuid"},
	{Name: "jsonb", Group: "JSON", Spelling: "jsonb"},
	{Name: "json", Group: "JSON", Spelling: "json"},
	{Name: "bytea", Group: "Binary", Spelling: "bytea"},
	{Name: "inet", Group: "Network", Spelling: "inet"},
	{Name: "cidr", Group: "Network", Spelling: "cidr"},
	{Name: "text[]", Group: "Arrays", Spelling: "text[]"},
	{Name: "integer[]", Group: "Arrays", Spelling: "integer[]"},
	{Name: "tsvector", Group: "Other", Spelling: "tsvector"},
}
