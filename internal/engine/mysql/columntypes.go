package mysql

import "github.com/felipemalacarne/mesa/internal/engine"

// columnTypes are the types the designer offers for MySQL, in the spellings
// the catalog reports.
var columnTypes = []engine.ColumnType{
	{Name: "bigint unsigned", Group: "Numeric", Spelling: "bigint unsigned", Integer: true},
	{Name: "bigint", Group: "Numeric", Spelling: "bigint", Integer: true},
	{Name: "int unsigned", Group: "Numeric", Spelling: "int unsigned", Integer: true},
	{Name: "int", Group: "Numeric", Spelling: "int", Integer: true},
	{Name: "smallint unsigned", Group: "Numeric", Spelling: "smallint unsigned", Integer: true},
	{Name: "smallint", Group: "Numeric", Spelling: "smallint", Integer: true},
	{Name: "tinyint unsigned", Group: "Numeric", Spelling: "tinyint unsigned", Integer: true},
	{Name: "tinyint", Group: "Numeric", Spelling: "tinyint", Integer: true},
	{Name: "decimal", Group: "Numeric", Args: "precision", Spelling: "decimal(12,2)"},
	{Name: "double", Group: "Numeric", Spelling: "double"},
	{Name: "float", Group: "Numeric", Spelling: "float"},
	{Name: "varchar", Group: "Text", Args: "length", Spelling: "varchar(255)"},
	{Name: "char", Group: "Text", Args: "length", Spelling: "char(36)"},
	{Name: "text", Group: "Text", Spelling: "text"},
	{Name: "mediumtext", Group: "Text", Spelling: "mediumtext"},
	{Name: "longtext", Group: "Text", Spelling: "longtext"},
	{Name: "enum", Group: "Text", Args: "values", Spelling: "enum('a','b')"},
	{Name: "set", Group: "Text", Args: "values", Spelling: "set('a','b')"},
	{Name: "tinyint(1)", Group: "Boolean", Spelling: "tinyint(1)"},
	{Name: "timestamp", Group: "Date & time", Spelling: "timestamp"},
	{Name: "datetime", Group: "Date & time", Spelling: "datetime"},
	{Name: "date", Group: "Date & time", Spelling: "date"},
	{Name: "time", Group: "Date & time", Spelling: "time"},
	{Name: "year", Group: "Date & time", Spelling: "year"},
	{Name: "json", Group: "JSON", Spelling: "json"},
	{Name: "binary", Group: "Binary", Args: "length", Spelling: "binary(16)"},
	{Name: "varbinary", Group: "Binary", Args: "length", Spelling: "varbinary(255)"},
	{Name: "blob", Group: "Binary", Spelling: "blob"},
	{Name: "longblob", Group: "Binary", Spelling: "longblob"},
}
