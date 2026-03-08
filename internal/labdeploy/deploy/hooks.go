package deploy

import (
	"bytes"
	"log/slog"
	"net/http"
	"text/template"
	"time"

	tmpl "github.com/jdillenberger/arastack/internal/labdeploy/template"

	"github.com/jdillenberger/arastack/pkg/executil"
)

// executeHooks runs lifecycle hooks, logging errors but not failing the deploy.
func executeHooks(hooks []tmpl.Hook, values map[string]string, runner *executil.Runner) {
	for _, hook := range hooks {
		switch hook.Type {
		case "exec":
			cmd, err := renderTemplate(hook.Command, values)
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
			resp.Body.Close()
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
