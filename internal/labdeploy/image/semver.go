package image

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// semverRe matches semantic version tags like v1.2.3, 1.2.3, v1.2.3-beta.
var semverRe = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)(-[a-zA-Z0-9.]+)?$`)

// SemVer represents a parsed semantic version.
type SemVer struct {
	Major int
	Minor int
	Patch int
	Pre   string // pre-release suffix, e.g. "beta", "rc.1"
}

// String returns the normalized version string (without v prefix).
func (v SemVer) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// ParseSemver parses a version tag into a SemVer struct.
func ParseSemver(tag string) (SemVer, error) {
	m := semverRe.FindStringSubmatch(tag)
	if m == nil {
		return SemVer{}, fmt.Errorf("not a semver tag: %s", tag)
	}

	parts := strings.Split(m[1], ".")
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])

	pre := ""
	if m[2] != "" {
		pre = m[2][1:] // strip leading "-"
	}

	return SemVer{Major: major, Minor: minor, Patch: patch, Pre: pre}, nil
}

// CompareSemver compares two SemVer values.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareSemver(a, b SemVer) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	if a.Pre == "" && b.Pre == "" {
		return 0
	}
	if a.Pre != "" && b.Pre == "" {
		return -1
	}
	if a.Pre == "" && b.Pre != "" {
		return 1
	}
	if a.Pre < b.Pre {
		return -1
	}
	if a.Pre > b.Pre {
		return 1
	}
	return 0
}

// UpgradeType returns "patch", "minor", or "major" based on the difference between two versions.
func UpgradeType(from, to SemVer) string {
	if to.Major != from.Major {
		return "major"
	}
	if to.Minor != from.Minor {
		return "minor"
	}
	return "patch"
}
