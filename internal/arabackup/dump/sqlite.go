package dump

// SQLiteDriver handles SQLite dumps.
type SQLiteDriver struct{}

func (d *SQLiteDriver) Name() string { return "sqlite" }

func (d *SQLiteDriver) DumpCommand(opts DumpOptions) []string {
	db := opts.Database
	if db == "" {
		db = "/data/db.sqlite"
	}
	return []string{"sqlite3", db, ".dump"}
}

func (d *SQLiteDriver) RestoreCommand(opts RestoreOptions) []string {
	db := opts.Database
	if db == "" {
		db = "/data/db.sqlite"
	}
	return []string{"sqlite3", db}
}

func (d *SQLiteDriver) FileExtension() string { return "sql" }

func (d *SQLiteDriver) Validate(labels map[string]string) error { return nil }
