# aradashboard

Web dashboard for arastack. aradashboard aggregates data from all other arastack services and presents it in a unified web interface.

## How It Works

aradashboard runs as a web server that:

1. **Discovers apps**: Scans aradeploy's apps directory and reads `.aradeploy.yaml` state files to build the list of deployed applications.
2. **Monitors health**: Polls araalert's `/api/app-health` endpoint to get container health status and caches the results.
3. **Queries services**: Fetches data from other arastack APIs:
   - **arascanner** → fleet/peer list and online status
   - **arabackup** → backup status and schedules
   - **araalert** → alert rules and history
4. **Collects stats**: Gathers CPU, memory, and disk usage per app via system metrics.
5. **Serves the UI**: Renders HTML templates with the aggregated data and serves static assets.

## Commands

```
aradashboard run                # Start web server
```

## Configuration

File: `/etc/arastack/config/aradashboard.yaml`

```yaml
server:
  port: 8420
  bind: 0.0.0.0

auth:
  password: ""                  # set to require login (leave empty to disable)
  session_ttl_minutes: 1440     # session duration (default: 24h)

arascanner:
  url: http://127.0.0.1:7120

arabackup:
  url: http://127.0.0.1:7160

araalert:
  url: http://127.0.0.1:7150
```

Environment variable overrides use the `ARADASHBOARD_` prefix (e.g., `ARADASHBOARD_AUTH_PASSWORD=secret`).

## Web Routes

| Path | Description |
|------|-------------|
| `/login` | Login page (when auth is enabled) |
| `/` | Dashboard overview |
| `/apps` | App management |
| `/backup` | Backup status |
| `/alerts` | Alert history |
| `/fleet` | Peer list |
| `/settings` | Configuration |
| `/api/*` | API endpoints for frontend data |

## Global Flags

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Debug logging |
| `--config <path>` | Config file path |

## Interactions with Other Tools

- **aradeploy**: Reads app directories and `.aradeploy.yaml` state files to list deployed applications. Also reads aradeploy config for `apps_dir` and Docker settings.
- **arascanner**: Queries `/api/peers` to display the fleet overview with peer status.
- **arabackup**: Queries `/api/status` to show backup schedules and last run times.
- **araalert**: Queries `/api/app-health` for health status, `/api/rules` and `/api/history` for alert information.
