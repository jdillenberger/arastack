package dump

// MariaDBDriver is an alias for MySQLDriver. MariaDB uses the same
// mysqldump/mysql CLI tools and is wire-compatible with MySQL.
type MariaDBDriver struct{ MySQLDriver }

func (d *MariaDBDriver) Name() string { return "mariadb" }
