package connection

import "strings"

type Driver string

const (
	PostgresDriver Driver = "postgres"
	MySQLDriver    Driver = "mysql"
)

func NewDriver(driver string) (*Driver, error) {
	d := Driver(strings.ToLower(driver))
	if !d.IsValid() {
		return nil, ErrInvalidDriver
	}

	return &d, nil
}

func (d *Driver) IsValid() bool {
	return *d == PostgresDriver || *d == MySQLDriver
}
