package mysql

import (
	"database/sql"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/value"
	"github.com/felipemalacarne/mesa/internal/engine"
)

// kindOfType maps a MySQL type name (as the driver reports it, or a
// column_type) to a value kind.
func kindOfType(t string) value.Kind {
	t = strings.ToUpper(strings.TrimSpace(t))
	if i := strings.IndexAny(t, "( "); i > 0 {
		t = t[:i]
	}
	switch t {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT", "YEAR":
		return value.KindInt
	case "FLOAT", "DOUBLE", "REAL":
		return value.KindFloat
	case "DECIMAL", "NUMERIC":
		return value.KindDecimal
	case "CHAR", "VARCHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT", "ENUM", "SET":
		return value.KindText
	case "BINARY", "VARBINARY", "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BIT", "GEOMETRY":
		return value.KindBytes
	case "JSON":
		return value.KindJSON
	case "DATE", "DATETIME", "TIMESTAMP", "TIME":
		return value.KindTime
	case "BOOL", "BOOLEAN":
		return value.KindBool
	}
	return value.KindOther
}

// decode turns the text the server sent into a value.
func decode(kind value.Kind, typ string, raw sql.RawBytes) value.Value {
	if raw == nil {
		return value.Null().WithType(typ)
	}
	text := string(raw)
	switch kind {
	case value.KindText:
		return value.Text(text).WithType(typ)
	case value.KindJSON:
		return value.JSON(text).WithType(typ)
	case value.KindBytes:
		return value.Bytes(append([]byte(nil), raw...)).WithType(typ)
	case value.KindTime:
		// DATETIME and TIMESTAMP are read in the session's UTC time zone.
		if strings.HasPrefix(typ, "datetime") || strings.HasPrefix(typ, "timestamp") {
			if t, err := time.Parse("2006-01-02 15:04:05.999999", text); err == nil {
				return value.Time(t.UTC()).WithType(typ)
			}
		}
		return value.TimeText(text).WithType(typ)
	}
	v, err := value.ParseText(kind, typ, text)
	if err != nil {
		return value.Other(typ, text)
	}
	return v
}

// encode turns a value into a driver argument.
func encode(v value.Value) any {
	switch v.Kind {
	case value.KindNull:
		return nil
	case value.KindBool:
		return v.AsBool()
	case value.KindInt:
		if t := v.Text(); len(t) > 18 {
			return t
		}
		return v.AsInt()
	case value.KindFloat:
		return v.AsFloat()
	case value.KindBytes:
		return v.AsBytes()
	case value.KindTime:
		if t := v.AsTime(); !t.IsZero() {
			return t.UTC().Format("2006-01-02 15:04:05.999999")
		}
		return strings.TrimSuffix(strings.Replace(v.Text(), "T", " ", 1), "Z")
	}
	return v.Text()
}

func encodeArgs(args []value.Value) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = encode(a)
	}
	return out
}

// readRows collects at most max rows (0 = all) of a result.
func readRows(rows *sql.Rows, max int) (*engine.Result, error) {
	cts, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	res := &engine.Result{Columns: make([]value.Column, len(cts)), Rows: []value.Row{}}
	kinds := make([]value.Kind, len(cts))
	for i, ct := range cts {
		typ := strings.ToLower(ct.DatabaseTypeName())
		if strings.HasPrefix(typ, "unsigned ") {
			typ = strings.TrimPrefix(typ, "unsigned ") + " unsigned"
		}
		kinds[i] = kindOfType(typ)
		res.Columns[i] = value.Column{Name: ct.Name(), Type: typ, Kind: kinds[i]}
	}
	dest := make([]any, len(cts))
	raw := make([]sql.RawBytes, len(cts))
	for i := range dest {
		dest[i] = &raw[i]
	}
	for rows.Next() {
		if max > 0 && len(res.Rows) >= max {
			res.Truncated = true
			break
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := make(value.Row, len(cts))
		for i := range raw {
			row[i] = decode(kinds[i], res.Columns[i].Type, raw[i])
		}
		res.Rows = append(res.Rows, row)
	}
	return res, rows.Err()
}
