package doctor

import (
	"github.com/jdillenberger/arastack/pkg/doctor"
	"github.com/jdillenberger/arastack/pkg/mdns"
)

// CheckAll runs all dependency and system checks.
func CheckAll() []doctor.CheckResult {
	return mdns.CheckAllDependencies()
}

// Fix attempts to install a missing dependency or fix system config.
func Fix(result doctor.CheckResult) error {
	return mdns.FixDependency(result)
}
