# aradashboard

Web-based monitoring dashboard for the AraStack homelab. Aggregates information from all other AraStack services into a unified UI.

## Commands

| Command | Description |
|---------|-------------|
| `aradashboard run` | Start the web server |

## Configuration

Default config path: `/etc/arastack/config/aradashboard.yaml`

| Key | Default | Description |
|-----|---------|-------------|
| `server.bind` | `0.0.0.0` | Bind address |
| `server.port` | `8420` | Web server port |
| `aradeploy.config` | `/etc/arastack/config/aradeploy.yaml` | Path to aradeploy config |
| `docker.runtime` | `docker` | Container runtime |
| `docker.compose_command` | `docker compose` | Compose command |
| `routing.enabled` | `true` | Enable routing features |
| `routing.https_enabled` | `true` | Enable HTTPS |
| `web.nav_color` | - | Navigation bar color |
| `services.peer_scanner.url` | `http://localhost:7120` | arascanner URL |
| `services.peer_scanner.secret` | - | PSK for arascanner auth |
| `services.araalert.url` | `http://127.0.0.1:7150` | araalert URL |
| `services.arabackup.url` | `http://127.0.0.1:7160` | arabackup URL |
| `ca.cert_path` | - | CA certificate path |

## Web Routes

| Route | Description |
|-------|-------------|
| `/` | Dashboard homepage |
| `/apps` | List deployed apps |
| `/apps/:name` | App detail page |
| `/apps/:name/logs` | App log viewer |
| `/fleet` | Fleet management page |
| `/backups` | Backup status page |
| `/alerts` | Alert rules and history |
| `/settings` | Settings page |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/stats` | Dashboard statistics |
| `GET` | `/api/apps` | List all apps |
| `GET` | `/api/apps/health` | Health status of all apps |
| `GET` | `/api/fleet` | Fleet information |
| `GET` | `/api/alerts/rules` | Alert rules |
| `GET` | `/api/alerts/history` | Alert history |
| `GET` | `/api/routing/status` | Routing status |

## Interactions with Other Tools

aradashboard is the central monitoring hub that connects to most other AraStack services:

- **aradeploy** - reads its config to discover deployed apps, display container status, and stream logs.
- **arascanner** - queries the `/api/peers` endpoint (authenticated via PSK) to display fleet peer information on the fleet page.
- **araalert** - queries alert rules and history via the araalert REST API to display on the alerts page.
- **arabackup** - queries backup status via the arabackup REST API to display on the backups page.
