package dump

// MariaDBDriver handles MariaDB dumps. Recent MariaDB images ship
// mariadb-dump / mariadb / mariadb-admin instead of the legacy
// mysqldump / mysql / mysqladmin symlinks.
type MariaDBDriver struct{}

func (d *MariaDBDriver) Name() string { return "mariadb" }

func (d *MariaDBDriver) DumpCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}

	if opts.PasswordEnv != "" {
		if opts.Database == "" || opts.Database == "all" {
			return []string{"sh", "-c", "exec mariadb-dump -u \"$1\" -p\"$" + opts.PasswordEnv + "\" --all-databases --add-drop-table", "--", user}
		}
		return []string{"sh", "-c", "exec mariadb-dump -u \"$1\" -p\"$" + opts.PasswordEnv + "\" --add-drop-table \"$2\"", "--", user, opts.Database}
	}

	args := []string{"mariadb-dump", "-u", user, "--add-drop-table"}
	if opts.Database == "" || opts.Database == "all" {
		args = append(args, "--all-databases")
	} else {
		args = append(args, opts.Database)
	}
	return args
}

func (d *MariaDBDriver) RestoreCommand(opts RestoreOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}

	if opts.PasswordEnv != "" {
		return []string{"sh", "-c", "exec mariadb -u \"$1\" -p\"$" + opts.PasswordEnv + "\"", "--", user}
	}

	return []string{"mariadb", "-u", user}
}

func (d *MariaDBDriver) ReadyCommand(opts DumpOptions) []string {
	user := opts.User
	if user == "" {
		user = "root"
	}
	if opts.PasswordEnv != "" {
		return []string{"sh", "-c", "exec mariadb-admin -u \"$1\" -p\"$" + opts.PasswordEnv + "\" ping", "--", user}
	}
	return []string{"mariadb-admin", "-u", user, "ping"}
}

func (d *MariaDBDriver) PreRestoreCommand(opts RestoreOptions) []string { return nil }

func (d *MariaDBDriver) FileExtension() string { return "sql" }

func (d *MariaDBDriver) Validate(labels map[string]string) error { return nil }
