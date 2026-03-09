package docker

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	"github.com/jdillenberger/arastack/pkg/executil"
)

const composeDefaultTimeout = 5 * time.Minute

// Compose wraps the docker compose CLI for read-only operations.
type Compose struct {
	runner  *executil.Runner
	command string // e.g. "docker compose"
}

// NewCompose creates a new Compose wrapper.
func NewCompose(runner *executil.Runner, composeCommand string) *Compose {
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
	fullArgs = append(fullArgs, "-f", filepath.Join(projectDir, aradeployconfig.ComposeFileName))
	fullArgs = append(fullArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.runner.RunWithContext(ctx, bin, fullArgs...)
}

// PSJson returns the status of containers in JSON format for structured parsing.
func (c *Compose) PSJson(projectDir string) (*executil.Result, error) {
	return c.run(projectDir, composeDefaultTimeout, "ps", "--format", "json")
}

// Logs streams logs to the given writer.
func (c *Compose) Logs(projectDir string, w io.Writer, follow bool, lines int) error {
	bin, baseArgs := c.cmdParts()
	fullArgs := make([]string, 0, len(baseArgs)+7)
	fullArgs = append(fullArgs, baseArgs...)
	fullArgs = append(fullArgs, "-f", filepath.Join(projectDir, aradeployconfig.ComposeFileName), "logs", "--no-color")
	if follow {
		fullArgs = append(fullArgs, "-f")
	}
	if lines > 0 {
		fullArgs = append(fullArgs, "-n", fmt.Sprintf("%d", lines))
	}
	return c.runner.RunStream(w, bin, fullArgs...)
}
