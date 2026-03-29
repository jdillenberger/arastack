package certs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// GenerateCABundle creates a CA certificate bundle combining system CAs,
// the local CA, and any peer CA certificates. The bundle is written to
// {dataDir}/ca-bundle.crt.
func GenerateCABundle(localCACertPath string, peerCACerts []string, dataDir string) error {
	systemBundle, err := os.ReadFile("/etc/ssl/certs/ca-certificates.crt") // #nosec G304 -- well-known system path
	if err != nil {
		return fmt.Errorf("reading system CA bundle: %w", err)
	}

	// Start with system CAs.
	size := len(systemBundle) + 1024 // rough estimate for additional CAs
	bundle := make([]byte, 0, size)
	bundle = append(bundle, systemBundle...)

	// Add local CA.
	localCA, err := os.ReadFile(localCACertPath) // #nosec G304 -- path is constructed internally
	if err != nil {
		slog.Warn("Local CA cert not available, using system bundle only", "path", localCACertPath, "error", err)
	} else {
		bundle = append(bundle, '\n')
		bundle = append(bundle, localCA...)
	}

	// Add peer CAs.
	for _, pem := range peerCACerts {
		if pem == "" {
			continue
		}
		bundle = append(bundle, '\n')
		bundle = append(bundle, []byte(pem)...)
	}

	bundlePath := filepath.Join(dataDir, "ca-bundle.crt")
	return os.WriteFile(bundlePath, bundle, 0o644) // #nosec G306 -- CA bundle is public; path is constructed internally
}
