package connection

import "strings"

// DefaultValue represents a default value for a column.
type DefaultValue struct {
	Value string
}

func NewDefaultValue(val string) DefaultValue {
	trimmed := strings.TrimSpace(val)

	return DefaultValue{Value: trimmed}
}

func (dv DefaultValue) IsEmpty() bool {
	return dv.Value == ""
}

func (dv DefaultValue) String() string {
	return dv.Value
}
