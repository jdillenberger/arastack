package dump

// PostgresDriver handles PostgreSQL dumps.
type PostgresDriver struct{}

func (d *PostgresDriver) Name() string { return "postgres" }

func (d *PostgresDriver) DumpCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "postgres"
	}

	if opts.Database == "" || opts.Database == "all" {
		return []string{"pg_dumpall", "-U", user}
	}
	return []string{"pg_dump", "-U", user, opts.Database}
}

func (d *PostgresDriver) RestoreCommand(opts RestoreOptions) []string {
	user := opts.User
	if user == "" {
		user = "postgres"
	}

	return []string{"psql", "-U", user}
}

func (d *PostgresDriver) FileExtension() string { return "sql" }

func (d *PostgresDriver) Validate(labels map[string]string) error { return nil }
