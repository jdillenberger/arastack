package dump

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/pkg/executil"
)

// Dumper orchestrates database dumps.
type Dumper struct {
	runner *executil.Runner
	cfg    *config.Config
}

// NewDumper creates a new Dumper.
func NewDumper(runner *executil.Runner, cfg *config.Config) *Dumper {
	return &Dumper{runner: runner, cfg: cfg}
}

// Dump executes a dump for a service and saves the output to the dump directory.
func (d *Dumper) Dump(app *discovery.App, svc discovery.ServiceBackupConfig) (string, error) {
	driver, err := d.resolveDriver(svc)
	if err != nil {
		return "", err
	}

	opts := DumpOptions{
		User:        svc.Labels.DumpUser,
		PasswordEnv: svc.Labels.DumpPasswordEnv,
		Database:    svc.Labels.DumpDatabase,
	}

	dumpCmd := driver.DumpCommand(opts)
	if len(dumpCmd) == 0 {
		return "", fmt.Errorf("driver %q returned empty dump command", driver.Name())
	}

	// Ensure dump directory exists
	dumpDir := d.cfg.DumpDir(app.Name)
	if err := os.MkdirAll(dumpDir, 0o750); err != nil {
		return "", fmt.Errorf("creating dump directory %s: %w", dumpDir, err)
	}

	// Generate dump filename
	timestamp := time.Now().Format("20060102-150405")
	ext := driver.FileExtension()
	filename := fmt.Sprintf("%s-%s.%s", driver.Name(), timestamp, ext)
	dumpPath := filepath.Join(dumpDir, filename)

	// Find the container name for docker exec
	containerName, err := d.findContainer(app, svc)
	if err != nil {
		return "", fmt.Errorf("finding container for %s/%s: %w", app.Name, svc.ServiceName, err)
	}

	// Build docker exec command that pipes to file
	// docker exec <container> <dump-command> > <dump-path>
	dockerArgs := append([]string{"exec", containerName}, dumpCmd...)

	// Open output file
	f, err := os.Create(dumpPath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return "", fmt.Errorf("creating dump file %s: %w", dumpPath, err)
	}
	defer f.Close() //nolint:errcheck // best-effort close after write

	if err := d.runner.RunPipe(f, nil, "docker", dockerArgs...); err != nil {
		// Clean up failed dump file
		_ = os.Remove(dumpPath)
		return "", fmt.Errorf("dump command failed for %s/%s: %w", app.Name, svc.ServiceName, err)
	}

	// Update latest symlink
	latestLink := filepath.Join(dumpDir, fmt.Sprintf("%s-latest.%s", driver.Name(), ext))
	_ = os.Remove(latestLink)
	_ = os.Symlink(filename, latestLink)

	slog.Info("Dump completed", "app", app.Name, "service", svc.ServiceName, "driver", driver.Name(), "file", dumpPath)

	// Clean up old dump files for this driver, keeping only the configured number
	if d.cfg.Dumps.Keep > 0 {
		d.cleanupOldDumps(dumpDir, driver.Name(), driver.FileExtension(), d.cfg.Dumps.Keep)
	}

	return dumpPath, nil
}

// WaitForReady polls the database readiness command until it succeeds or times out.
func (d *Dumper) WaitForReady(app *discovery.App, svc discovery.ServiceBackupConfig) error {
	driver, err := d.resolveDriver(svc)
	if err != nil {
		return err
	}

	opts := DumpOptions{
		User:        svc.Labels.DumpUser,
		PasswordEnv: svc.Labels.DumpPasswordEnv,
		Database:    svc.Labels.DumpDatabase,
	}

	readyCmd := driver.ReadyCommand(opts)
	if readyCmd == nil {
		// No health check available — fall back to a fixed sleep.
		slog.Info("No health check for driver, waiting 10s", "driver", driver.Name(), "service", svc.ServiceName)
		time.Sleep(10 * time.Second)
		return nil
	}

	containerName, err := d.findContainer(app, svc)
	if err != nil {
		return fmt.Errorf("finding container for %s/%s: %w", app.Name, svc.ServiceName, err)
	}

	const timeout = 60 * time.Second
	const interval = 1 * time.Second
	deadline := time.Now().Add(timeout)

	dockerArgs := append([]string{"exec", containerName}, readyCmd...)
	for {
		_, err := d.runner.Run("docker", dockerArgs...)
		if err == nil {
			slog.Info("Database ready", "app", app.Name, "service", svc.ServiceName)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database %s/%s not ready after %s", app.Name, svc.ServiceName, timeout)
		}
		time.Sleep(interval)
	}
}

// Restore imports a dump file into the database container.
func (d *Dumper) Restore(app *discovery.App, svc discovery.ServiceBackupConfig, dumpFile string) error {
	driver, err := d.resolveDriver(svc)
	if err != nil {
		return err
	}

	opts := RestoreOptions{
		User:        svc.Labels.DumpUser,
		PasswordEnv: svc.Labels.DumpPasswordEnv,
		Database:    svc.Labels.DumpDatabase,
		FilePath:    dumpFile,
	}

	containerName, err := d.findContainer(app, svc)
	if err != nil {
		return fmt.Errorf("finding container for %s/%s: %w", app.Name, svc.ServiceName, err)
	}

	// Execute pre-restore command if the driver provides one (e.g. rm existing SQLite DB).
	if preCmd := driver.PreRestoreCommand(opts); preCmd != nil {
		preArgs := append([]string{"exec", containerName}, preCmd...)
		if _, err := d.runner.Run("docker", preArgs...); err != nil {
			return fmt.Errorf("pre-restore command failed for %s/%s: %w", app.Name, svc.ServiceName, err)
		}
	}

	restoreCmd := driver.RestoreCommand(opts)
	if len(restoreCmd) == 0 {
		return fmt.Errorf("driver %q returned empty restore command", driver.Name())
	}

	// Pipe dump file contents to docker exec stdin instead of using sh -c
	f, err := os.Open(dumpFile) // #nosec G304 -- path is constructed internally
	if err != nil {
		return fmt.Errorf("opening dump file %s: %w", dumpFile, err)
	}
	defer f.Close() //nolint:errcheck // read-only file

	dockerArgs := append([]string{"exec", "-i", containerName}, restoreCmd...)
	_, err = d.runner.RunPipeStdin(f, "docker", dockerArgs...)
	if err != nil {
		return fmt.Errorf("restore command failed for %s/%s: %w", app.Name, svc.ServiceName, err)
	}

	slog.Info("Restore completed", "app", app.Name, "service", svc.ServiceName, "file", dumpFile)
	return nil
}

// LatestDump returns the path to the latest dump file for a service.
func (d *Dumper) LatestDump(app *discovery.App, svc discovery.ServiceBackupConfig) (string, error) {
	driver, err := d.resolveDriver(svc)
	if err != nil {
		return "", err
	}

	dumpDir := d.cfg.DumpDir(app.Name)
	ext := driver.FileExtension()
	latestLink := filepath.Join(dumpDir, fmt.Sprintf("%s-latest.%s", driver.Name(), ext))

	target, err := os.Readlink(latestLink)
	if err != nil {
		return "", fmt.Errorf("no latest dump found for %s/%s: %w", app.Name, svc.ServiceName, err)
	}

	// Resolve relative symlink
	if !filepath.IsAbs(target) {
		target = filepath.Join(dumpDir, target)
	}

	return target, nil
}

// resolveDriver returns the appropriate driver for a service.
func (d *Dumper) resolveDriver(svc discovery.ServiceBackupConfig) (Driver, error) {
	driverName := svc.Labels.DumpDriver
	if driverName == "" {
		return nil, fmt.Errorf("no dump driver specified for service %q", svc.ServiceName)
	}

	if driverName == "custom" {
		return NewCustomDriver(
			svc.Labels.DumpCommand,
			svc.Labels.DumpRestoreCommand,
			svc.Labels.DumpFileExt,
		), nil
	}

	return Get(driverName)
}

// cleanupOldDumps removes old dump files for a driver, keeping only the most recent `keep` files.
// Symlinks (e.g. *-latest.*) are excluded from counting and removal.
func (d *Dumper) cleanupOldDumps(dumpDir, driverName, ext string, keep int) {
	entries, err := os.ReadDir(dumpDir)
	if err != nil {
		return
	}

	prefix := driverName + "-"
	suffix := "." + ext
	latestSuffix := "-latest" + suffix

	var dumpFiles []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) && !strings.HasSuffix(name, latestSuffix) {
			if e.Type().IsRegular() {
				dumpFiles = append(dumpFiles, name)
			}
		}
	}

	if len(dumpFiles) <= keep {
		return
	}

	// Sort lexicographically — timestamp format ensures chronological order
	sort.Strings(dumpFiles)

	for _, name := range dumpFiles[:len(dumpFiles)-keep] {
		path := filepath.Join(dumpDir, name)
		if err := os.Remove(path); err != nil {
			slog.Warn("Failed to remove old dump file", "path", path, "error", err)
		} else {
			slog.Debug("Removed old dump file", "path", path)
		}
	}
}

// findContainer finds the running container name for a service in a compose project.
func (d *Dumper) findContainer(app *discovery.App, svc discovery.ServiceBackupConfig) (string, error) {
	// docker compose uses <project>-<service>-1 naming convention
	// The project name is typically the directory name
	projectName := app.Name

	result, err := d.runner.Run("docker", "compose", "-p", projectName,
		"-f", filepath.Join(app.AppDir, "docker-compose.yml"),
		"ps", "--format", "{{.Name}}", svc.ServiceName)
	if err != nil {
		fallback := projectName + "-" + svc.ServiceName + "-1"
		slog.Warn("Could not query container name via docker compose, falling back to convention-based name",
			"app", app.Name, "service", svc.ServiceName, "fallback", fallback, "error", err)
		return fallback, nil
	}

	name := strings.TrimSpace(result.Stdout)
	if name == "" {
		fallback := projectName + "-" + svc.ServiceName + "-1"
		slog.Warn("Docker compose returned empty container name, falling back to convention-based name",
			"app", app.Name, "service", svc.ServiceName, "fallback", fallback)
		return fallback, nil
	}

	// Take first line if multiple
	if idx := strings.Index(name, "\n"); idx > 0 {
		name = name[:idx]
	}

	return name, nil
}
