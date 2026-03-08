package compose

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/pkg/executil"
)

const (
	defaultTimeout = 5 * time.Minute
	longTimeout    = 10 * time.Minute
)

// Compose wraps the docker compose CLI.
type Compose struct {
	runner  *executil.Runner
	command string // e.g. "docker compose"
}

// New creates a new Compose wrapper.
func New(runner *executil.Runner, composeCommand string) *Compose {
	return &Compose{
		runner:  runner,
		command: composeCommand,
	}
}

func (c *Compose) cmdParts() (bin string, args []string) {
	parts := strings.Fields(c.command)
	if len(parts) == 0 {
		return "docker", []string{"compose"}
	}
	return parts[0], parts[1:]
}

func (c *Compose) run(projectDir string, timeout time.Duration, args ...string) (*executil.Result, error) {
	bin, baseArgs := c.cmdParts()
	fullArgs := make([]string, 0, len(baseArgs)+len(args)+2)
	fullArgs = append(fullArgs, baseArgs...)
	fullArgs = append(fullArgs, "-f", projectDir+"/docker-compose.yml")
	fullArgs = append(fullArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.runner.RunWithContext(ctx, bin, fullArgs...)
}

// Up runs docker compose up -d.
func (c *Compose) Up(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, longTimeout, "up", "-d", "--remove-orphans")
}

// Down runs docker compose down.
func (c *Compose) Down(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, defaultTimeout, "down")
}

// Start runs docker compose start.
func (c *Compose) Start(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, defaultTimeout, "start")
}

// Stop runs docker compose stop.
func (c *Compose) Stop(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, defaultTimeout, "stop")
}

// Restart runs docker compose restart.
func (c *Compose) Restart(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, defaultTimeout, "restart")
}

// PS returns the status of containers in the project.
func (c *Compose) PS(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, defaultTimeout, "ps", "--format", "table")
}

// PSJson returns the status of containers in JSON format for structured parsing.
func (c *Compose) PSJson(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, defaultTimeout, "ps", "--format", "json")
}

// Logs streams logs to the given writer.
func (c *Compose) Logs(projectDir string, w io.Writer, follow bool, lines int) error {
	bin, baseArgs := c.cmdParts()
	fullArgs := make([]string, 0, len(baseArgs)+7)
	fullArgs = append(fullArgs, baseArgs...)
	fullArgs = append(fullArgs, "-f", projectDir+"/docker-compose.yml", "logs", "--no-color")
	if follow {
		fullArgs = append(fullArgs, "-f")
	}
	if lines > 0 {
		fullArgs = append(fullArgs, "-n", fmt.Sprintf("%d", lines))
	}
	return c.runner.RunStream(w, bin, fullArgs...)
}

// Build runs docker compose build.
func (c *Compose) Build(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, longTimeout, "build")
}

// UpWithBuild runs docker compose up -d --build.
func (c *Compose) UpWithBuild(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, longTimeout, "up", "-d", "--build", "--remove-orphans")
}

// Pull pulls the latest images.
func (c *Compose) Pull(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, longTimeout, "pull")
}
