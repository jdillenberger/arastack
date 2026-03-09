# Deploying Apps

This guide explains how aradeploy works under the hood — from templates to running containers — and covers customization, routing, data storage, and backup integration.

## Overview

aradeploy is a template engine for Docker Compose. It takes an app template (a `docker-compose.yml` template + metadata), collects your configuration values, renders the final compose file, and runs it. Everything else — backups, monitoring, DNS — hooks in automatically via docker-compose labels.

## The Template System

### Where Templates Come From

Templates live in git repositories. By default, aradeploy uses the [arastack-templates](https://github.com/jdillenberger/arastack-templates) repository. You can add more:

```bash
aradeploy repos add https://github.com/youruser/my-templates.git
aradeploy repos list        # see all configured repos
aradeploy repos update      # pull latest changes
```

Templates are cloned to `~/.aradeploy/repos/` and cached locally.

### Template Structure

Each template is a directory containing:

```
nextcloud/
├── app.yaml                        # Metadata: name, values, ports, routing, etc.
└── docker-compose.yml.tmpl         # Go template for docker-compose.yml
    (or docker-compose.yml)         # Static compose file (no templating)
```

### app.yaml — Template Metadata

The `app.yaml` file defines everything about the app:

```yaml
name: nextcloud
description: Self-hosted file sync and share
category: productivity
version: "1.0.0"

# Values the user must or can provide
values:
  - name: admin_password
    description: "Nextcloud admin password"
    secret: true
    auto_gen: password            # auto-generate a 32-char password
  - name: web_port
    description: "Web UI port"
    default: "8080"
  - name: db_password
    description: "Database password"
    secret: true
    auto_gen: password
  - name: upload_limit
    description: "Max upload size"
    default: "10G"

# Port mappings
ports:
  - host: 8080
    container: 80
    value_name: web_port          # links to the "web_port" value above

# Named volumes
volumes:
  - name: data
    container: /var/www/html

# Reverse proxy routing
routing:
  enabled: true
  subdomain: nextcloud
  container_port: 80

# Info shown after deploy
post_deploy_info:
  access_url: "http://{{.hostname}}.{{.domain}}:{{.web_port}}"
  credentials: "admin / {{.admin_password}}"
  notes:
    - "First load may take 30-60 seconds"
```

### Template Rendering

Templates use Go's `text/template` syntax. Available variables include all user-defined values plus system values:

| Variable | Source |
|----------|--------|
| `{{.hostname}}` | System hostname |
| `{{.domain}}` | Network domain from config |
| `{{.app_name}}` | The app name |
| `{{.data_dir}}` | App data directory path |
| `{{.web_port}}` | Web port (if defined) |
| `{{.routing_domain}}` | Full routing domain |
| Any value from `app.yaml` | User input or default/auto-generated |

## Deployment Flow

When you run `aradeploy deploy nextcloud`:

```
1. Load template from registry
   └── app.yaml + docker-compose.yml.tmpl

2. Collect values
   ├── Interactive wizard (or --quick for defaults)
   ├── Values file (-f values.yaml)
   └── CLI overrides (--set key=value)

3. Generate secrets
   └── Auto-generate passwords, UUIDs for secret values

4. Render docker-compose.yml
   └── Go template + values → final compose file

5. Inject Traefik labels (if routing enabled)
   └── Adds routing labels to the main service

6. Create Docker network (if needed)
   └── Default: aradeploy-net

7. Deploy Traefik (if routing enabled and not running)
   └── Auto-deploys Traefik as a managed app

8. Run docker compose up -d
   └── Starts all containers

9. Save state
   └── /opt/aradeploy/apps/<app>/.aradeploy.yaml

10. Run post-deploy hooks (if any)
    └── HTTP calls, scripts, etc.
```

## Where Data Is Stored

### App Files

Each deployed app gets a directory under `/opt/aradeploy/apps/`:

```
/opt/aradeploy/apps/nextcloud/
├── docker-compose.yml            # Rendered compose file
├── .env                          # Environment variables (if any)
└── .aradeploy.yaml               # Deployment state (name, version, timestamp)
```

### App Data

Persistent data (databases, uploads, configs) is stored under `/opt/aradeploy/data/`:

```
/opt/aradeploy/data/nextcloud/    # Mounted into containers as volumes
```

This separation means you can remove and redeploy an app without losing data (unless you use `--purge-data`).

### Backups

If the app template includes backup labels:

```
/mnt/backup/borg/nextcloud/       # Borg repository (deduplicated archives)
/opt/arabackup/dumps/nextcloud/   # Database dumps (timestamped files)
```

## Reverse Proxy with Traefik

When routing is enabled (default), aradeploy automatically:

1. **Deploys Traefik** as a managed container if it's not already running
2. **Injects labels** into your app's `docker-compose.yml` for Traefik to discover
3. **Configures HTTPS** via Let's Encrypt if you've set an ACME email

### How Routing Works

Traefik watches Docker for containers with specific labels. aradeploy injects these labels based on the template's `routing` section:

```yaml
# In app.yaml:
routing:
  enabled: true
  subdomain: nextcloud
  container_port: 80
  websocket: false                # set true for apps with WebSocket
  keep_ports: false               # set true to also expose host ports
```

This produces labels like:

```yaml
labels:
  traefik.enable: "true"
  traefik.http.routers.nextcloud.rule: "Host(`nextcloud.home.local`)"
  traefik.http.services.nextcloud.loadbalancer.server.port: "80"
```

The domain is built from `<subdomain>.<routing.domain>` in your aradeploy config.

### HTTPS

To enable HTTPS with automatic certificates:

```yaml
# /etc/arastack/config/aradeploy.yaml
routing:
  enabled: true
  domain: home.example.com       # your real domain
  https:
    enabled: true
    acme_email: admin@example.com
```

Traefik will use Let's Encrypt to issue certificates for each app's subdomain.

### Local Network Access (mDNS)

On the local network, aramdns watches for Traefik routing labels and publishes the domains via Avahi mDNS. This means `nextcloud.home.local` resolves automatically on any device connected to the same LAN — no DNS server configuration needed.

## Deploying Nextcloud — Full Example

### Quick Deploy

```bash
aradeploy deploy nextcloud --quick --yes
```

This uses all default values and auto-generates secrets.

### Custom Deploy

```bash
aradeploy deploy nextcloud
```

The wizard asks for each value. You can also provide a values file:

```yaml
# my-nextcloud-values.yaml
admin_password: "my-secure-password"
web_port: "9090"
upload_limit: "20G"
```

```bash
aradeploy deploy nextcloud -f my-nextcloud-values.yaml --yes
```

### After Deployment

Check the status:

```bash
aradeploy status nextcloud       # container status
aradeploy logs nextcloud -f      # follow logs
```

The rendered compose file is at `/opt/aradeploy/apps/nextcloud/docker-compose.yml` — you can inspect it to see exactly what's running.

### Nextcloud Data

```
/opt/aradeploy/data/nextcloud/   # Nextcloud files, database, config
```

Backups (if enabled in template):
```
/mnt/backup/borg/nextcloud/      # Nightly Borg archives
/opt/arabackup/dumps/nextcloud/  # Database dumps
```

## Deploying WordPress — Full Example

```bash
aradeploy deploy wordpress
```

WordPress typically needs:
- A MySQL/MariaDB database (included in the template)
- Admin credentials (auto-generated or user-provided)
- A web port

The template handles all of this. After deployment:

```bash
aradeploy status wordpress
# Open http://<hostname>.local:<port> or http://wordpress.home.local
```

WordPress data:
```
/opt/aradeploy/data/wordpress/   # WordPress files + database data
```

## Customizing Templates

### Override a Template Locally

Export a template to customize it:

```bash
aradeploy templates export nextcloud
```

This copies the template to `~/.aradeploy/templates/nextcloud/`. Local templates take priority over repo templates. Edit the `docker-compose.yml.tmpl` or `app.yaml` as needed.

### Create Your Own Template

```bash
aradeploy templates new myapp
```

This scaffolds a new template directory. Add a `--dockerfile` flag to include a Dockerfile.

### Validate Templates

```bash
aradeploy templates lint              # lint all templates
aradeploy templates lint myapp        # lint a specific one
```

## Backup Integration

App templates can include backup labels that arabackup reads automatically:

```yaml
# In docker-compose.yml (or template)
services:
  app:
    labels:
      arabackup.enable: "true"
      arabackup.borg.paths: "data"              # paths relative to data_dir
      arabackup.dump.driver: "postgres"          # postgres, mysql, mongodb, sqlite, custom
      arabackup.dump.user: "postgres"
      arabackup.dump.password-env: "POSTGRES_PASSWORD"
      arabackup.dump.database: "nextcloud"
```

arabackup discovers these labels when scanning deployed apps and includes them in the backup schedule. No additional configuration needed.

See [arabackup docs](arabackup.md) for the full label reference and supported database drivers.

## App Lifecycle

```bash
aradeploy deploy <app>              # Deploy
aradeploy stop <app>                # Stop containers
aradeploy start <app>               # Start containers
aradeploy restart <app>             # Restart containers
aradeploy update <app>              # Pull latest images, recreate
aradeploy remove <app>              # Remove (keeps data)
aradeploy remove <app> --purge-data # Remove everything
```

### Checking for Updates

```bash
aradeploy outdated                  # Check all apps for newer images
aradeploy outdated nextcloud        # Check a specific app
aradeploy upgrade nextcloud         # Upgrade to latest image version
aradeploy upgrade --all             # Upgrade everything
```

### Export for Standalone Use

If you want to take your apps out of arastack and manage them with plain Docker Compose:

```bash
aradeploy eject -o ./my-apps
```

This exports all deployed apps into the output directory, with each app in its own subdirectory containing its rendered `docker-compose.yml` and `.env`. You can then manage them with `docker compose` directly.

## Configuration Reference

See `/etc/arastack/config/aradeploy.yaml`:

```yaml
hostname: myhost                    # used in domain names
apps_dir: /opt/aradeploy/apps       # where app compose files live
data_dir: /opt/aradeploy/data       # where app data volumes live

network:
  domain: local                     # domain suffix
  web_port: 8080                    # default web port

docker:
  runtime: docker                   # docker or podman
  compose_command: docker compose   # compose command
  default_network: aradeploy-net    # shared Docker network

routing:
  enabled: true                     # enable Traefik reverse proxy
  provider: traefik
  domain: ""                        # defaults to hostname.domain
  https:
    enabled: false
    acme_email: ""                  # required for Let's Encrypt

araalert:
  url: http://127.0.0.1:7150       # for push events on update failures
```

Environment variable overrides use the `ARADEPLOY_` prefix.
