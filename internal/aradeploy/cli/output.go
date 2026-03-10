package cli

import "github.com/jdillenberger/arastack/pkg/cliutil"

// outputJSON marshals v as indented JSON to stdout.
func outputJSON(v interface{}) error {
	return cliutil.OutputJSON(v)
}
