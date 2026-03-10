// Package ports defines the default ports for all arastack services.
// This is the single source of truth — all services and tools reference
// these constants so that port numbers stay in sync.
package ports

import "fmt"

const (
	AraScanner   = 7120
	AraMonitor   = 7130
	AraNotify    = 7140
	AraAlert     = 7150
	AraBackup    = 7160
	AraDashboard = 8420
)

// DefaultURL returns "http://127.0.0.1:<port>" for the given port.
func DefaultURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
