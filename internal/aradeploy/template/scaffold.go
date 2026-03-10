package template

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldOptions controls what files are generated.
type ScaffoldOptions struct {
	Dockerfile bool
}

// Scaffold creates a skeleton template directory with app.yaml, docker-compose.yml.tmpl, etc.
func Scaffold(destDir, name string, opts ScaffoldOptions) error {
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("template %q already exists at %s", name, destDir)
	}

	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("creating template directory: %w", err)
	}

	requiresBuildLine := ""
	if opts.Dockerfile {
		requiresBuildLine = "\nrequires_build: true\n"
	}

	appYAML := fmt.Sprintf(`name: %s
description: "TODO: Add description"
category: "custom"
version: "1.0.0"
%s
ports:
  - host: 8080
    container: 8080
    protocol: tcp
    description: "Web UI"
    value_name: web_port

volumes:
  - name: data
    container: /data
    description: "Application data"

values:
  - name: web_port
    description: "Web UI port"
    default: "8080"
    required: true

health_check:
  url: "http://localhost:{{.web_port}}"
  interval: "30s"

# backup:
#   paths: []
#   pre_hook: ""
#   post_hook: ""

# post_deploy_info:
#   access_url: "http://{{.hostname}}.{{.domain}}:{{.web_port}}"
#   credentials: "See the app documentation"
#   notes:
#     - "Complete the initial setup wizard"

# hooks:
#   post_deploy:
#     - type: exec
#       command: "echo 'Deployed {{.app_name}}'"
`, name, requiresBuildLine)

	var composeTmpl string
	if opts.Dockerfile {
		composeTmpl = fmt.Sprintf(`services:
  %s:
    build: .
    container_name: %s
    restart: unless-stopped
    ports:
      - "{{.web_port}}:8080"
    volumes:
      - {{.data_dir}}/data:/data
    networks:
      - {{.network}}
    security_opt:
      - no-new-privileges:true
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    pids_limit: 256
    # mem_limit: 512m
    # cpus: 1.0

networks:
  {{.network}}:
    external: true
`, name, name)
	} else {
		composeTmpl = fmt.Sprintf(`services:
  %s:
    image: TODO_IMAGE:latest
    container_name: %s
    restart: unless-stopped
    ports:
      - "{{.web_port}}:8080"
    volumes:
      - {{.data_dir}}/data:/data
    networks:
      - {{.network}}
    security_opt:
      - no-new-privileges:true
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    pids_limit: 256
    # mem_limit: 512m
    # cpus: 1.0

networks:
  {{.network}}:
    external: true
`, name, name)
	}

	envTmpl := `# Environment variables for {{.app_name}}
TZ={{.timezone}}
`

	files := map[string]string{
		"app.yaml":                appYAML,
		"docker-compose.yml.tmpl": composeTmpl,
		".env.tmpl":               envTmpl,
	}

	if opts.Dockerfile {
		files["Dockerfile"] = `FROM alpine:3.21

# Install dependencies
# RUN apk add --no-cache <packages>

WORKDIR /app

# Copy application files
# COPY . .

EXPOSE 8080

CMD ["echo", "TODO: replace with your app command"]
`
		files[".dockerignore"] = `.git
.gitignore
*.md
.env
`
	}

	for fname, content := range files {
		fpath := filepath.Join(destDir, fname)
		if err := os.WriteFile(fpath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", fname, err)
		}
	}

	return nil
}
