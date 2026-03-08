package dump

// MySQLDriver handles MySQL/MariaDB dumps.
type MySQLDriver struct{}

func (d *MySQLDriver) Name() string { return "mysql" }

func (d *MySQLDriver) DumpCommand(opts DumpOptions) []string {
	args := []string{"mysqldump"}

	if opts.User != "" {
		args = append(args, "-u", opts.User)
	} else {
		args = append(args, "-u", "root")
	}

	if opts.PasswordEnv != "" {
		args = append(args, "-p$"+opts.PasswordEnv)
	}

	if opts.Database == "" || opts.Database == "all" {
		args = append(args, "--all-databases")
	} else {
		args = append(args, opts.Database)
	}

	return args
}

func (d *MySQLDriver) RestoreCommand(opts RestoreOptions) []string {
	args := []string{"mysql"}

	if opts.User != "" {
		args = append(args, "-u", opts.User)
	} else {
		args = append(args, "-u", "root")
	}

	if opts.PasswordEnv != "" {
		args = append(args, "-p$"+opts.PasswordEnv)
	}

	return args
}

func (d *MySQLDriver) FileExtension() string { return "sql" }

func (d *MySQLDriver) Validate(labels map[string]string) error { return nil }
