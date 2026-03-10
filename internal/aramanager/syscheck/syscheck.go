package syscheck

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"

	"github.com/jdillenberger/arastack/pkg/doctor"
)

const groupName = "arastack"

type dirSpec struct {
	path string
	mode os.FileMode
}

var dirs = []dirSpec{
	{"/etc/arastack/config", 0o2770},
	{"/opt/aradeploy/apps", 0o2775},
	{"/opt/aradeploy/data", 0o2775},
	{"/var/lib/arascanner", 0o2775},
	{"/var/lib/aranotify", 0o2775},
	{"/var/lib/araalert", 0o2775},
	{"/var/lib/arabackup", 0o2775},
}

// CheckAll returns check results for the arastack group, user membership, and core directories.
func CheckAll() []doctor.CheckResult {
	var results []doctor.CheckResult

	// Check group exists
	grp, grpErr := user.LookupGroup(groupName)
	results = append(results, doctor.CheckResult{
		Name:      "arastack-group",
		Installed: grpErr == nil,
	})

	// Check current user is in the group
	inGroup := false
	if grpErr == nil {
		inGroup = userInGroup(grp.Gid)
	}
	results = append(results, doctor.CheckResult{
		Name:      "user-in-group",
		Installed: inGroup,
	})

	// Check directories
	for _, d := range dirs {
		results = append(results, checkDir(d))
	}

	return results
}

// Fix fixes a single failing check result.
func Fix(r doctor.CheckResult) error {
	if r.Installed {
		return nil
	}

	switch {
	case r.Name == "arastack-group":
		return sudoRun("groupadd", groupName)
	case r.Name == "user-in-group":
		username := os.Getenv("SUDO_USER")
		if username == "" {
			username = os.Getenv("USER")
		}
		if username == "" || username == "root" {
			u, err := user.Current()
			if err != nil {
				return fmt.Errorf("cannot determine current user: %w", err)
			}
			username = u.Username
		}
		return sudoRun("usermod", "-aG", groupName, username)
	case strings.HasPrefix(r.Name, "dir:"):
		path := strings.TrimPrefix(r.Name, "dir:")
		mode := modeForPath(path)
		if err := sudoRun("mkdir", "-p", path); err != nil {
			return err
		}
		if err := sudoRun("chown", "root:"+groupName, path); err != nil {
			return err
		}
		return sudoRun("chmod", fmt.Sprintf("%o", mode), path)
	}

	return fmt.Errorf("unknown check: %s", r.Name)
}

func checkDir(d dirSpec) doctor.CheckResult {
	name := "dir:" + d.path
	info, err := os.Stat(d.path)
	if err != nil {
		return doctor.CheckResult{Name: name}
	}

	if !info.IsDir() {
		return doctor.CheckResult{Name: name, Version: "not a directory"}
	}

	// Check group ownership
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return doctor.CheckResult{Name: name, Version: "cannot read ownership"}
	}

	grp, err := user.LookupGroup(groupName)
	if err != nil {
		return doctor.CheckResult{Name: name, Version: "group missing"}
	}

	var gid uint32
	if _, err := fmt.Sscanf(grp.Gid, "%d", &gid); err != nil {
		return doctor.CheckResult{Name: name, Version: fmt.Sprintf("invalid gid %q", grp.Gid)}
	}

	if stat.Gid != gid {
		return doctor.CheckResult{Name: name, Version: fmt.Sprintf("wrong group (gid %d)", stat.Gid)}
	}

	// Check mode (including setgid bit)
	perm := info.Mode().Perm() | (info.Mode() & os.ModeSetgid)
	wantPerm := os.FileMode(d.mode & 0o7777)
	// Convert setgid: os.ModeSetgid is a high bit in Go's FileMode
	actualPerm := perm
	if info.Mode()&os.ModeSetgid != 0 {
		actualPerm = perm | os.ModeSetgid
	}
	_ = actualPerm

	// Compare the raw unix mode bits
	rawMode := os.FileMode(stat.Mode) & 0o7777
	if rawMode != wantPerm {
		return doctor.CheckResult{Name: name, Version: fmt.Sprintf("mode %04o, want %04o", rawMode, wantPerm)}
	}

	return doctor.CheckResult{Name: name, Installed: true, Version: fmt.Sprintf("%04o root:%s", rawMode, groupName)}
}

func userInGroup(gid string) bool {
	u, err := user.Current()
	if err != nil {
		return false
	}

	// Check if it's the user's primary group
	if u.Gid == gid {
		return true
	}

	// Check supplementary groups from /etc/group
	f, err := os.Open("/etc/group")
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 4)
		if len(parts) < 4 {
			continue
		}
		if parts[2] != gid {
			continue
		}
		members := strings.Split(parts[3], ",")
		for _, m := range members {
			if m == u.Username {
				return true
			}
		}
	}

	return false
}

func modeForPath(path string) os.FileMode {
	for _, d := range dirs {
		if d.path == path {
			return d.mode
		}
	}
	return 0o2775
}

func sudoRun(args ...string) error {
	cmd := exec.CommandContext(context.Background(), "sudo", args...) // #nosec G204 G702 -- command is from trusted config
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
