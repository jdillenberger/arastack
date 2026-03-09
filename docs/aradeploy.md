# aradeploy

Template-based Docker Compose deployment and management tool. Deploys applications from templates with variable substitution, secret generation, and optional Traefik routing integration.

## Commands

| Command | Description |
|---------|-------------|
| `aradeploy deploy <app>` | Deploy app from template (`--values`, `--set`, `--dry-run`, `--quick`) |
| `aradeploy remove <app>` | Remove deployed app (`--purge-data`, `--force`) |
| `aradeploy start <app>` | Start a deployed app |
| `aradeploy stop <app>` | Stop a deployed app |
| `aradeploy restart <app>` | Restart a deployed app |
| `aradeploy status [app]` | Show container status |
| `aradeploy logs <app>` | Show app logs (`--follow`, `--lines`) |
| `aradeploy list` | List apps (`--all`, `--filter`, `--category`) |
| `aradeploy info <app>` | Show app template details |
| `aradeploy update [app]` | Pull latest images and recreate (`--all`) |
| `aradeploy templates` | Manage app templates (list, create, export, lint) |
| `aradeploy repos` | Manage template source repositories (add/remove/update git repos) |
| `aradeploy export <app>` | Export app configuration |
| `aradeploy eject <app>` | Remove deployment templating |
| `aradeploy upgrade <app>` | Upgrade app to new version |
| `aradeploy pin <app>` | Pin app to specific version |
| `aradeploy prune` | Cleanup dangling volumes and networks |
| `aradeploy config` | Configuration management |

## Configuration

Default config path: `/etc/arastack/config/aradeploy.yaml`

| Key | Default | Description |
|-----|---------|-------------|
| `hostname` | - | Machine hostname |
| `apps_dir` | `/opt/aradeploy/apps` | Directory where apps are deployed |
| `data_dir` | `/opt/aradeploy/data` | Directory for app data volumes |
| `templates_dir` | `~/.aradeploy/templates` | Local templates directory |
| `network.domain` | `local` | Domain suffix |
| `network.web_port` | `8080` | Web port for routing |
| `docker.runtime` | `docker` | Container runtime |
| `docker.compose_command` | `docker compose` | Compose command |
| `docker.default_network` | `aradeploy-net` | Default Docker network |
| `routing.enabled` | `true` | Enable routing features |
| `routing.provider` | `traefik` | Routing provider |
| `routing.domain` | - | Custom routing domain |
| `routing.https.enabled` | `true` | Enable HTTPS |
| `routing.https.acme_email` | - | ACME email for Let's Encrypt |
| `araalert.url` | `http://127.0.0.1:7150` | araalert URL for pushing update-failed events |

## How It Works

1. Templates define apps as parameterized Docker Compose files with values, secrets, and metadata.
2. `aradeploy deploy` renders a template with user-provided or auto-generated values.
3. The rendered `docker-compose.yml` is placed in the apps directory and started.
4. Traefik labels are optionally added for automatic reverse proxy routing.
5. App metadata (version, deploy time, values) is tracked alongside the compose file.

## Interactions with Other Tools

- **arabackup** - reads aradeploy's app directory and compose files to discover what to back up. Backup labels in compose services define backup behavior.
- **araalert** - reads aradeploy config to discover deployed apps for health monitoring (`app-down` rules). Also receives `update-failed` events (with retry on failure) when `aradeploy update --all` or `aradeploy upgrade --all` encounters failures. Configured via `araalert.url`.
- **aradashboard** - reads aradeploy config to list and display deployed apps, their status, and logs.
- **aramdns** - watches containers deployed by aradeploy for Traefik labels and publishes their `.local` domains via mDNS.
