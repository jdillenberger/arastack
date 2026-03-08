package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExtractBinaryFromTarGz extracts a named binary from a tar.gz stream.
func ExtractBinaryFromTarGz(r io.Reader, name string) ([]byte, error) {
	result, err := ExtractBinariesFromTarGz(r, []string{name})
	if err != nil {
		return nil, err
	}
	data, ok := result[name]
	if !ok {
		return nil, fmt.Errorf("binary %q not found in archive", name)
	}
	return data, nil
}

// ExtractBinariesFromTarGz extracts multiple named binaries from a tar.gz stream.
// Returns a map of name -> data for each found binary.
func ExtractBinariesFromTarGz(r io.Reader, names []string) (map[string][]byte, error) {
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("opening gzip: %w", err)
	}
	defer gz.Close() //nolint:errcheck // read-only decompressor

	result := make(map[string][]byte)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}
		if wanted[hdr.Name] {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading %s from archive: %w", hdr.Name, err)
			}
			result[hdr.Name] = data
			if len(result) == len(wanted) {
				break
			}
		}
	}
	return result, nil
}

// ReplaceBinary atomically replaces the binary at dst with the one at src.
func ReplaceBinary(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return os.Chmod(dst, 0o755)
	}

	// Cross-device fallback: copy to a temp file on the same filesystem as dst, then rename.
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
	}
	defer in.Close() //nolint:errcheck // read-only file

	dstDir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dstDir, filepath.Base(dst)+".update-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dstDir, err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("copying update: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file to %s: %w", dst, err)
	}
	return nil
}

// MapArch maps Go GOARCH values to release asset architecture names.
func MapArch(goarch string) string {
	switch goarch {
	case "arm":
		return "armv7"
	default:
		return goarch
	}
}
