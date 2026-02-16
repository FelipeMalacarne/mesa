package connection

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidDataType = errors.New("invalid base data type")

// DataType encapsula o tipo e seus modificadores de tamanho/precisão
type DataType struct {
	baseType  string
	length    *int // Ex: 255 para VARCHAR(255)
	precision *int // Ex: 2 para DECIMAL(10,2) (junto com length=10)
}

func NewDataType(baseType string, length, precision *int) (DataType, error) {
	base := strings.ToUpper(strings.TrimSpace(baseType))

	// Aqui o domínio pode ter uma lista de tipos permitidos, se desejar restringir
	if base == "" {
		return DataType{}, ErrInvalidDataType
	}

	return DataType{
		baseType:  base,
		length:    length,
		precision: precision,
	}, nil
}

// Format retorna a string pronta para o SQL (ex: "VARCHAR(255)")
func (dt DataType) Format() string {
	if dt.length != nil && dt.precision != nil {
		return fmt.Sprintf("%s(%d,%d)", dt.baseType, *dt.length, *dt.precision)
	}
	if dt.length != nil {
		return fmt.Sprintf("%s(%d)", dt.baseType, *dt.length)
	}
	return dt.baseType
}
