package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireLock takes an exclusive file lock to prevent concurrent operations on the same app.
func acquireLock(appsDir, appName string) (*os.File, error) {
	lockDir := filepath.Join(appsDir, ".locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}
	lockPath := filepath.Join(lockDir, appName+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("app %s: another operation is in progress", appName)
	}
	return f, nil
}

// releaseLock releases the file lock and removes the lock file.
func releaseLock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	name := f.Name()
	f.Close()
	os.Remove(name)
}
