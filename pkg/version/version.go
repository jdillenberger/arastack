package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is the build version, set via ldflags.
	Version = "dev"
	// Commit is the build commit, set via ldflags.
	Commit = "none"
	// Date is the build date, set via ldflags.
	Date = "unknown"
)

// SetInfo sets version information from ldflags.
func SetInfo(v, c, d string) {
	Version = v
	Commit = c
	Date = d
}

// NewCommand returns a cobra command that prints version info for the given tool.
func NewCommand(toolName string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s %s (commit: %s, built: %s)\n", toolName, Version, Commit, Date)
		},
	}
}
