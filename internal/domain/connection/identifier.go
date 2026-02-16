package connection

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidIdentifier = errors.New("identifier contains invalid characters or is too long")
	// Regex padrão para nomes no Postgres/MySQL (letras, números, underscore)
	validIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)
)

// Identifier é um Value Object que garante um nome válido para bancos, schemas, tabelas e colunas.
type Identifier struct {
	value string
}

func NewIdentifier(val string) (Identifier, error) {
	if !validIdentifierRegex.MatchString(val) {
		return Identifier{}, ErrInvalidIdentifier
	}

	return Identifier{value: val}, nil
}

func (i Identifier) String() string {
	return i.value
}

func (i Identifier) IsValid() bool {
	return validIdentifierRegex.MatchString(i.value)
}

func (i Identifier) Quoted() string {
	return fmt.Sprintf(`"%s"`, i.value)
}

func MustNewIdentifier(val string) Identifier {
	result, err := NewIdentifier(val)
	if err != nil {
		panic(fmt.Sprintf("invalid identifier: %s", val))
	}

	return result
}
