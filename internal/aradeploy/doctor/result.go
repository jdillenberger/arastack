package doctor

// Severity indicates the urgency of a check result.
type Severity int

const (
	// SeverityOK means the check passed.
	SeverityOK Severity = iota
	// SeverityWarn means the check found a non-critical issue.
	SeverityWarn
	// SeverityFail means the check found a critical issue.
	SeverityFail
)

// Category identifies a group of related deployment checks.
type Category string

const (
	CategoryDomains    Category = "domains"
	CategoryLabels     Category = "labels"
	CategoryCerts      Category = "certs"
	CategoryContainers Category = "containers"
	CategoryEnvVars    Category = "envvars"
)

// AllCategories returns all deployment check categories in execution order.
func AllCategories() []Category {
	return []Category{
		CategoryDomains,
		CategoryLabels,
		CategoryCerts,
		CategoryContainers,
		CategoryEnvVars,
	}
}

// DeployCheckResult holds the result of a single deployment consistency check.
type DeployCheckResult struct {
	Category Category     `json:"category"`
	App      string       `json:"app,omitempty"`
	Name     string       `json:"name"`
	Severity Severity     `json:"severity"`
	Detail   string       `json:"detail,omitempty"`
	Fixable  bool         `json:"fixable,omitempty"`
	FixFunc  func() error `json:"-"`
}
