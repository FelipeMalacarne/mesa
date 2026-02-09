// Package inspector provides a factory for creating inspectors for different database drivers.
package inspector

import (
	"fmt"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/felipemalacarne/mesa/internal/infrastructure/postgres"
)

type Factory struct {
	inspectors map[connection.Driver]connection.Inspector
}

func NewFactory() *Factory {
	return &Factory{
		inspectors: map[connection.Driver]connection.Inspector{
			connection.PostgresDriver: postgres.NewInspector(),
		},
	}
}

func (f *Factory) ForDriver(driver connection.Driver) (connection.Inspector, error) {
	inspector, ok := f.inspectors[driver]
	if !ok {
		return nil, fmt.Errorf("no inspector registered for driver %s", driver)
	}

	return inspector, nil
}
