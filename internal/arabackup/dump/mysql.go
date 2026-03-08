package dump

// MySQLDriver handles MySQL/MariaDB dumps.
type MySQLDriver struct{}

func (d *MySQLDriver) Name() string { return "mysql" }

func (d *MySQLDriver) DumpCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}

	cmd := "mysqldump -u " + user
	if opts.PasswordEnv != "" {
		cmd += " -p\"$" + opts.PasswordEnv + "\""
	}

	if opts.Database == "" || opts.Database == "all" {
		cmd += " --all-databases"
	} else {
		cmd += " " + opts.Database
	}

	return []string{"sh", "-c", cmd}
}

func (d *MySQLDriver) RestoreCommand(opts RestoreOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}

	cmd := "mysql -u " + user
	if opts.PasswordEnv != "" {
		cmd += " -p\"$" + opts.PasswordEnv + "\""
	}

	return []string{"sh", "-c", cmd}
}

func (d *MySQLDriver) FileExtension() string { return "sql" }

func (d *MySQLDriver) Validate(labels map[string]string) error { return nil }
