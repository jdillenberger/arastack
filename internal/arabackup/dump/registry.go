package dump

import "fmt"

var registry = map[string]Driver{}

// Register adds a driver to the registry.
func Register(d Driver) {
	registry[d.Name()] = d
}

// Get returns a driver by name.
func Get(name string) (Driver, error) {
	d, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown dump driver: %q", name)
	}
	return d, nil
}

func init() {
	Register(&PostgresDriver{})
	Register(&MySQLDriver{})
	Register(&MariaDBDriver{})
	Register(&MongoDBDriver{})
	Register(&SQLiteDriver{})
}
