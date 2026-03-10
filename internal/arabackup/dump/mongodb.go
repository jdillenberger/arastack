package dump

// MongoDBDriver handles MongoDB dumps.
type MongoDBDriver struct{}

func (d *MongoDBDriver) Name() string { return "mongodb" }

func (d *MongoDBDriver) DumpCommand(opts DumpOptions) []string {
	args := []string{"mongodump", "--archive"}

	if opts.User != "" {
		args = append(args, "--username", opts.User)
	}
	if opts.Database != "" && opts.Database != "all" {
		args = append(args, "--db", opts.Database)
	}

	return args
}

func (d *MongoDBDriver) RestoreCommand(opts RestoreOptions) []string {
	args := []string{"mongorestore", "--archive", "--drop"}

	if opts.User != "" {
		args = append(args, "--username", opts.User)
	}

	return args
}

func (d *MongoDBDriver) ReadyCommand(opts DumpOptions) []string {
	return []string{"mongosh", "--eval", "db.runCommand('ping')"}
}

func (d *MongoDBDriver) PreRestoreCommand(opts RestoreOptions) []string { return nil }

func (d *MongoDBDriver) FileExtension() string { return "archive" }

func (d *MongoDBDriver) Validate(labels map[string]string) error { return nil }
