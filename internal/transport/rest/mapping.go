package rest

import (
	"github.com/felipemalacarne/mesa/internal/application/commands"
	"github.com/felipemalacarne/mesa/internal/transport/rest/contract"
)

func mapTableColumns(cols []contract.CreateTableColumn) []commands.TableColumn {
	result := make([]commands.TableColumn, len(cols))
	for i, col := range cols {
		result[i] = commands.TableColumn{
			Name:         col.Name,
			Type:         string(col.Type),
			Length:       col.Length,
			Precision:    col.Precision,
			Nullable:     ptrToBool(col.Nullable),
			PrimaryKey:   ptrToBool(col.PrimaryKey),
			DefaultValue: col.DefaultValue,
		}
	}
	return result
}

func mapTableIndexes(idxs *[]contract.CreateTableIndex) []commands.TableIndex {
	if idxs == nil {
		return nil
	}
	result := make([]commands.TableIndex, len(*idxs))
	for i, idx := range *idxs {
		var method string
		if idx.Method != nil {
			method = string(*idx.Method)
		}
		result[i] = commands.TableIndex{
			Name:    idx.Name,
			Columns: idx.Columns,
			Unique:  ptrToBool(idx.Unique),
			Method:  method,
		}
	}
	return result
}

func ptrToBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
