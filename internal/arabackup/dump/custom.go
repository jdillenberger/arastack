package dump

import (
	"fmt"
	"strings"
)

// CustomDriver handles custom dump commands specified via labels.
type CustomDriver struct {
	command        string
	restoreCommand string
	fileExt        string
}

// NewCustomDriver creates a custom driver from label values.
func NewCustomDriver(command, restoreCommand, fileExt string) *CustomDriver {
	if fileExt == "" {
		fileExt = "dump"
	}
	return &CustomDriver{
		command:        command,
		restoreCommand: restoreCommand,
		fileExt:        fileExt,
	}
}

func (d *CustomDriver) Name() string { return "custom" }

func (d *CustomDriver) DumpCommand(opts DumpOptions) []string {
	return splitCommand(d.command)
}

func (d *CustomDriver) RestoreCommand(opts RestoreOptions) []string {
	return splitCommand(d.restoreCommand)
}

func (d *CustomDriver) ReadyCommand(opts DumpOptions) []string { return nil }

func (d *CustomDriver) PreRestoreCommand(opts RestoreOptions) []string { return nil }

func (d *CustomDriver) FileExtension() string { return d.fileExt }

func (d *CustomDriver) Validate(labels map[string]string) error {
	if labels["arabackup.dump.command"] == "" {
		return fmt.Errorf("custom driver requires arabackup.dump.command label")
	}
	return nil
}

// splitCommand splits a command string into args (simple space-split).
func splitCommand(cmd string) []string {
	return strings.Fields(cmd)
}
