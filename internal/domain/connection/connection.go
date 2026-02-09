// Package connection contains the Connection entity and related logic.
package connection

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidDriver = errors.New("supported drivers are: postgresql, mysql")
	ErrInvalidPort   = errors.New("port must be between 1 and 65535")
)

type Connection struct {
	ID        uuid.UUID
	Name      string
	Driver    Driver
	Host      string
	Port      int
	Username  string
	Password  string // Já deve chegar aqui criptografada pela camada de application
	UpdatedAt time.Time
	CreatedAt time.Time
}

// NewConnection é o nosso Factory Method (Construtor com validação)
func NewConnection(name, driver, host string, port int, user, encryptedPass string) (*Connection, error) {
	validatedDriver, err := NewDriver(driver)
	if err != nil {
		return nil, err
	}

	if port <= 0 || port > 65535 {
		return nil, ErrInvalidPort
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return &Connection{
		ID:        uuid,
		Name:      strings.TrimSpace(name),
		Driver:    *validatedDriver,
		Host:      host,
		Port:      port,
		Username:  user,
		Password:  encryptedPass,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}, nil
}
