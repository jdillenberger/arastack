package dump

// MySQLDriver handles MySQL/MariaDB dumps.
type MySQLDriver struct{}

func (d *MySQLDriver) Name() string { return "mysql" }

func (d *MySQLDriver) DumpCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}

	if opts.PasswordEnv != "" {
		// Use sh -c for env var expansion, passing user/db as positional args to avoid injection.
		if opts.Database == "" || opts.Database == "all" {
			return []string{"sh", "-c", "exec mysqldump -u \"$1\" -p\"$" + opts.PasswordEnv + "\" --all-databases --add-drop-table", "--", user}
		}
		return []string{"sh", "-c", "exec mysqldump -u \"$1\" -p\"$" + opts.PasswordEnv + "\" --add-drop-table \"$2\"", "--", user, opts.Database}
	}

	args := []string{"mysqldump", "-u", user, "--add-drop-table"}
	if opts.Database == "" || opts.Database == "all" {
		args = append(args, "--all-databases")
	} else {
		args = append(args, opts.Database)
	}
	return args
}

func (d *MySQLDriver) RestoreCommand(opts RestoreOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}

	if opts.PasswordEnv != "" {
		return []string{"sh", "-c", "exec mysql -u \"$1\" -p\"$" + opts.PasswordEnv + "\"", "--", user}
	}

	return []string{"mysql", "-u", user}
}

func (d *MySQLDriver) ReadyCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}
	if opts.PasswordEnv != "" {
		return []string{"sh", "-c", "exec mysqladmin -u \"$1\" -p\"$" + opts.PasswordEnv + "\" ping", "--", user}
	}
	return []string{"mysqladmin", "-u", user, "ping"}
}

func (d *MySQLDriver) PreRestoreCommand(opts RestoreOptions) []string { return nil }

func (d *MySQLDriver) FileExtension() string { return "sql" }

func (d *MySQLDriver) Validate(labels map[string]string) error { return nil }
