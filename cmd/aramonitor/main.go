package main

import (
	"os"

	"github.com/jdillenberger/arastack/internal/aramonitor/cli"
	"github.com/jdillenberger/arastack/pkg/version"
)

// Set by goreleaser via ldflags.
var (
	ver    = "dev"
	commit = "none"
	date   = "unknown"
)

func main() {
	version.SetInfo(ver, commit, date)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
