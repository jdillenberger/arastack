package lint

import (
	"github.com/jdillenberger/arastack/internal/labdeploy/template"
)

// Linter inspects templates for common issues.
type Linter struct {
	registry *template.Registry
}

// NewLinter creates a new Linter backed by the given registry.
func NewLinter(registry *template.Registry) *Linter {
	return &Linter{registry: registry}
}

// LintAll lints all templates and returns a combined result.
func (l *Linter) LintAll() *Result {
	var allFindings []Finding
	for _, name := range l.registry.List() {
		findings := l.LintTemplate(name)
		allFindings = append(allFindings, findings...)
	}
	return buildResult(allFindings)
}

// LintTemplate lints a single template and returns its findings.
func (l *Linter) LintTemplate(name string) []Finding {
	meta, ok := l.registry.Get(name)
	if !ok {
		return []Finding{{
			Template: name,
			File:     "app.yaml",
			Severity: SeverityError,
			Check:    "template-not-found",
			Message:  "Template not found in registry",
		}}
	}

	var findings []Finding

	findings = append(findings, l.lintAppYAML(name, meta)...)

	dummyValues := l.buildDummyValues(meta)

	findings = append(findings, l.lintCompose(name, meta, dummyValues)...)

	findings = append(findings, l.lintValueUsage(name, meta)...)

	if len(meta.LintIgnore) > 0 {
		findings = filterSuppressed(findings, meta.LintIgnore)
	}

	return findings
}

func buildResult(findings []Finding) *Result {
	s := CountSummary(findings)
	return &Result{Findings: findings, Summary: s}
}
