package gateway

import (
	"fmt"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/felipemalacarne/mesa/internal/infrastructure/postgres"
)

// Factory disponibiliza implementações de Gateway para cada driver suportado.
type Factory struct {
	gateways map[connection.Driver]connection.Gateway
}

func NewFactory() *Factory {
	return &Factory{
		gateways: map[connection.Driver]connection.Gateway{
			connection.PostgresDriver: postgres.NewGateway(),
		},
	}
}

func (f *Factory) ForDriver(driver connection.Driver) (connection.Gateway, error) {
	gw, ok := f.gateways[driver]
	if !ok {
		return nil, fmt.Errorf("no gateway registered for driver %s", driver)
	}

	return gw, nil
}
