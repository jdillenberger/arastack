package executil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Runner wraps os/exec with logging support.
type Runner struct {
	Verbose bool
}

// Result holds command execution results.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a command and returns captured output.
func (r *Runner) Run(name string, args ...string) (*Result, error) {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("command %q exited with code %d: %s", name, result.ExitCode, stderr.String())
	}
	if err != nil {
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}

// RunWithEnv executes a command with additional environment variables.
func (r *Runner) RunWithEnv(env []string, name string, args ...string) (*Result, error) {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("command %q exited with code %d: %s", name, result.ExitCode, stderr.String())
	}
	if err != nil {
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}

// RunWithContext executes a command with context support for cancellation/timeouts.
func (r *Runner) RunWithContext(ctx context.Context, name string, args ...string) (*Result, error) {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if ctx.Err() != nil {
		return result, fmt.Errorf("command %q: %w", name, ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("command %q exited with code %d: %s", name, result.ExitCode, stderr.String())
	}
	if err != nil {
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}

// RunInteractive runs a command with stdin/stdout/stderr attached.
func (r *Runner) RunInteractive(name string, args ...string) error {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunStream runs a command streaming stdout to the given writer.
func (r *Runner) RunStream(w io.Writer, name string, args ...string) error {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunPipe runs a command and pipes stdout to the given writer, with custom env.
func (r *Runner) RunPipe(w io.Writer, env []string, name string, args ...string) error {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunWithEnvAndDir executes a command with additional environment variables and
// working directory.
func (r *Runner) RunWithEnvAndDir(env []string, dir, name string, args ...string) (*Result, error) {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("command %q exited with code %d: %s", name, result.ExitCode, stderr.String())
	}
	if err != nil {
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}

// RunPipeStdin runs a command with stdin piped from the given reader and
// captures output. This avoids using sh -c for piping.
func (r *Runner) RunPipeStdin(stdin io.Reader, name string, args ...string) (*Result, error) {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, "exec: %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 -- runner is a general-purpose exec wrapper
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("command %q exited with code %d: %s", name, result.ExitCode, stderr.String())
	}
	if err != nil {
		return result, fmt.Errorf("command %q failed: %w", name, err)
	}

	return result, nil
}
