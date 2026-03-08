package lint

// Severity represents the severity of a lint finding.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Finding represents a single lint finding.
type Finding struct {
	Template string   `json:"template"`
	File     string   `json:"file"`
	Severity Severity `json:"severity"`
	Check    string   `json:"check"`
	Message  string   `json:"message"`
}

// Result holds all findings from a lint run.
type Result struct {
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
}

// Summary counts findings by severity.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Total    int `json:"total"`
}

// CountSummary counts findings by severity.
func CountSummary(findings []Finding) Summary {
	var s Summary
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarning:
			s.Warnings++
		case SeverityInfo:
			s.Infos++
		}
		s.Total++
	}
	return s
}
