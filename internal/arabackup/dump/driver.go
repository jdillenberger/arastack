package dump

// DumpOptions holds options for a dump command.
type DumpOptions struct {
	User        string
	PasswordEnv string // env var name inside the container
	Database    string
}

// RestoreOptions holds options for a restore command.
type RestoreOptions struct {
	User        string
	PasswordEnv string
	Database    string
	FilePath    string // path to dump file (inside container or piped)
}

// Driver defines the interface for database dump drivers.
type Driver interface {
	Name() string
	DumpCommand(opts DumpOptions) []string
	RestoreCommand(opts RestoreOptions) []string
	// ReadyCommand returns a command to check if the database is ready.
	// Return nil if no health check is available (falls back to sleep).
	ReadyCommand(opts DumpOptions) []string
	// PreRestoreCommand returns a command to run before restoring a dump
	// (e.g. removing an existing SQLite database). Return nil to skip.
	PreRestoreCommand(opts RestoreOptions) []string
	FileExtension() string
	Validate(labels map[string]string) error
}
