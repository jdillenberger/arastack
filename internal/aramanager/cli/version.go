package cli

import (
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(version.NewCommand("aramanager"))
}
