package cliutil

import (
	"encoding/json"
	"fmt"
	"os"
)

// OutputJSON marshals v as indented JSON to stdout.
func OutputJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(data))
	return err
}
