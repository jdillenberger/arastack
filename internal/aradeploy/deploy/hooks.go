package deploy

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"text/template"
	"time"

	tmpl "github.com/jdillenberger/arastack/internal/aradeploy/template"

	"github.com/jdillenberger/arastack/pkg/executil"
)

// shellQuote returns a shell-safe single-quoted representation of s.
func shellQuote(s string) string {
	// Single-quote the value, escaping any embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// shellEscapeValues returns a copy of values with every value shell-quoted,
// preventing injection when the rendered string is passed to sh -c.
func shellEscapeValues(values map[string]string) map[string]string {
	safe := make(map[string]string, len(values))
	for k, v := range values {
		safe[k] = shellQuote(v)
	}
	return safe
}

// executeHooks runs lifecycle hooks, logging errors but not failing the deploy.
func executeHooks(hooks []tmpl.Hook, values map[string]string, runner *executil.Runner) {
	for _, hook := range hooks {
		switch hook.Type {
		case "exec":
			// Shell-escape all values before template rendering so that
			// user-supplied data cannot break out of the intended command.
			cmd, err := renderTemplate(hook.Command, shellEscapeValues(values))
			if err != nil {
				slog.Warn("Hook: failed to render command template", "error", err)
				continue
			}
			slog.Info("Running hook", "command", cmd)
			if _, err := runner.Run("sh", "-c", cmd); err != nil {
				slog.Warn("Hook failed", "command", cmd, "error", err)
			}
		case "http":
			url, err := renderTemplate(hook.URL, values)
			if err != nil {
				slog.Warn("Hook: failed to render URL template", "error", err)
				continue
			}
			method := hook.Method
			if method == "" {
				method = "GET"
			}
			var bodyReader *bytes.Reader
			if hook.Body != "" {
				body, err := renderTemplate(hook.Body, values)
				if err != nil {
					slog.Warn("Hook: failed to render body template", "error", err)
					continue
				}
				bodyReader = bytes.NewReader([]byte(body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}
			slog.Info("Running HTTP hook", "method", method, "url", url)
			req, err := http.NewRequest(method, url, bodyReader)
			if err != nil {
				slog.Warn("Hook: failed to create request", "error", err)
				continue
			}
			if hook.Body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				slog.Warn("HTTP hook failed", "url", url, "error", err)
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode >= 400 {
				slog.Warn("HTTP hook returned error", "url", url, "status", resp.StatusCode)
			}
		}
	}
}

// renderTemplate renders a Go template string with the given values.
func renderTemplate(tmplStr string, values map[string]string) (string, error) {
	t, err := template.New("").Parse(tmplStr)
	if err != nil {
		return tmplStr, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, values); err != nil {
		return tmplStr, err
	}
	return buf.String(), nil
}
