# aradeploy

Docker Compose application deployment manager. aradeploy deploys applications from templates, manages their lifecycle, handles container image versioning, and provides reverse proxy routing via Traefik.

## How It Works

aradeploy uses a **template system** where each application is defined as a template containing a `docker-compose.yml.tmpl`, an `app.yaml` metadata file, and optional static files. Templates live in git repositories that aradeploy clones and keeps updated.

When you deploy an app, aradeploy:

1. Loads the template from the registry (local overrides take priority over repo templates)
2. Merges user-provided values with template defaults and auto-generated values (passwords, UUIDs)
3. Renders Go templates to produce `docker-compose.yml`, `.env`, and any other config files
4. Creates the Docker network if it doesn't exist
5. Injects Traefik routing labels into the compose file (if routing is enabled)
6. Generates TLS certificates for the domain (if HTTPS is enabled)
7. Writes everything to `/opt/aradeploy/apps/<app>/`
8. Runs `docker compose up -d`
9. Saves deployment state to `.aradeploy.yaml` inside the app directory
10. Runs any post-deploy hooks defined in the template

File-based locking prevents concurrent operations on the same app.

## Commands

### App Management

```
aradeploy deploy <app>          # Deploy from template
  -f values.yaml                # Values file
  --set key=value               # Override individual values
  --dry-run                     # Preview without deploying
  --yes                         # Skip confirmation
  --quick                       # Skip interactive wizard

aradeploy remove <app>          # Stop and remove app
  --purge-data                  # Also delete data volumes
  --force                       # Skip confirmation

aradeploy start <app>           # Start containers
aradeploy stop <app>            # Stop containers
aradeploy restart <app>         # Restart containers
aradeploy status [app]          # Show container status
aradeploy logs <app>            # Stream logs
  -f                            # Follow
  -n <lines>                    # Tail lines
aradeploy list                  # List deployed apps
  --all                         # Include available templates
  --filter <text>               # Filter by name
  --category <cat>              # Filter by category
aradeploy info <app>            # Show template details
```

### Updates and Upgrades

`update` pulls the latest container images and recreates containers. It does not touch templates or configuration — it's a quick "refresh" for an already-deployed app.

`upgrade` is more comprehensive: it checks for newer template versions **and** newer container images, shows a diff of what will change, and applies both together. Use `upgrade` when you want to pick up template changes (new environment variables, updated compose structure) in addition to image updates.

```
aradeploy update <app>          # Pull latest images, recreate containers
  --all                         # Update all apps

aradeploy upgrade [app]         # Upgrade templates + container images
  --all                         # Upgrade all apps
  --dry-run                     # Preview changes
  --check                       # Check only, don't upgrade
  --patch-only                  # Only apply patch version bumps
aradeploy outdated [app]        # Check for newer image versions
aradeploy pin [app]             # Resolve floating tags to pinned semver
  --dry-run                     # Preview changes
  --update                      # Pin and pull
```

### Template Management

```
aradeploy templates list        # List all available templates
aradeploy templates export <t>  # Copy template locally for customization
aradeploy templates delete <t>  # Remove local template override
aradeploy templates path        # Print templates directory
aradeploy templates new <name>  # Scaffold a new template
  --dockerfile                  # Include a Dockerfile
aradeploy templates lint [t]    # Validate templates
```

### Repository Management

```
aradeploy repos list            # List template repositories
aradeploy repos add <url>       # Add git repo as template source
  --name <name>                 # Custom name
  --ref <branch|tag>            # Branch or tag to track
aradeploy repos remove <name>   # Remove repository
aradeploy repos update [name]   # Pull latest from repos
```

### Configuration

```
aradeploy config show           # Print current config
aradeploy config init           # Create default config
aradeploy config validate       # Check for errors
```

### Other

```
aradeploy export                # Export all deployed apps to YAML
aradeploy import <file>         # Import and deploy from export
  --dry-run                     # Preview
aradeploy eject [-o dir]        # Export compose + env for standalone use
aradeploy prune [--force]       # Clean Docker resources
aradeploy completion <shell>    # Shell completion (bash/zsh/fish)
```

## Configuration

File: `/etc/arastack/config/aradeploy.yaml`

```yaml
hostname: myhost
apps_dir: /opt/aradeploy/apps
data_dir: /opt/aradeploy/data
templates_dir: ~/.aradeploy/templates

network:
  domain: local
  web_port: 8080

docker:
  runtime: docker
  compose_command: docker compose
  default_network: aradeploy-net

routing:
  enabled: true
  provider: traefik
  domain: ""              # defaults to hostname.domain
  https:
    enabled: true
    acme_email: admin@example.com

araalert:
  url: http://127.0.0.1:7150
```

Environment variable overrides use the `ARADEPLOY_` prefix (e.g., `ARADEPLOY_APPS_DIR=/custom/path`).

## Template Format

Each template contains an `app.yaml` metadata file:

```yaml
name: myapp
description: My application
category: media
version: 1.0.0

values:
  - name: web_port
    description: "Web UI port"
    default: "8080"
    required: false
  - name: db_password
    description: "Database password"
    secret: true
    auto_gen: password     # auto-generate 32 hex chars

ports:
  - host: 8080
    container: 8080
    value_name: web_port

volumes:
  - name: data
    container: /data

routing:
  enabled: true
  subdomain: myapp
  container_port: 8080

hooks:
  post_deploy:
    - type: exec
      command: "echo deployed"

post_deploy_info:
  access_url: "http://{{.hostname}}.{{.domain}}:{{.web_port}}"
```

Templates use Go's `text/template` syntax. Available variables include all user values plus system values: `hostname`, `domain`, `app_name`, `data_dir`, `web_port`, `routing_domain`.

## Docker Compose Labels

aradeploy injects Traefik labels for routing and reads backup labels for arabackup integration:

```yaml
labels:
  # Backup integration (read by arabackup)
  arabackup.enable: "true"
  arabackup.dump.driver: "postgres"
  arabackup.dump.database: "mydb"
  arabackup.dump.user: "postgres"
  arabackup.dump.password-env: "POSTGRES_PASSWORD"
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path |
| `--apps-dir <path>` | Override apps directory |
| `-v`, `--verbose` | Debug logging |
| `-q`, `--quiet` | Suppress non-essential output |
| `--json` | JSON output |

## Interactions with Other Tools

- **arabackup**: Reads aradeploy's config and app directories to discover services with backup labels. arabackup parses docker-compose.yml files to find `arabackup.*` labels.
- **araalert**: aradeploy pushes `update-failed` events to araalert when container image updates fail. Events are spooled to disk for retry if araalert is unreachable.
- **aramdns**: Watches Docker containers for Traefik labels injected by aradeploy and publishes the domains via mDNS.
- **aradashboard**: Reads aradeploy state files (`.aradeploy.yaml`) to display deployed apps.

## Container Registry Support

aradeploy queries these registries for image version resolution:

- Docker Hub (`docker.io`)
- GitHub Container Registry (`ghcr.io`)
- LinuxServer (`lscr.io`)
- Quay.io

It uses registry v2 API with token authentication to resolve floating tags (like `latest`) to specific semver versions and detect available updates.

## File Locations

| Path | Purpose |
|------|---------|
| `/opt/aradeploy/apps/<app>/` | Deployed app files (compose, env, state) |
| `/opt/aradeploy/data/<app>/` | App data volumes |
| `~/.aradeploy/templates/` | Local template overrides |
| `~/.aradeploy/repos/` | Cloned template repositories |
| `~/.aradeploy/repos.yaml` | Repository manifest |
| `/var/lib/aradeploy/pending-events.json` | Event spool for araalert |
