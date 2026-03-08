package docker

import "os/exec"

// DetectRuntime returns the first available container runtime (docker or podman).
func DetectRuntime() string {
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
}
