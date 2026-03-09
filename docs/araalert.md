# araalert

Alert rule evaluation daemon. araalert periodically checks the health of deployed applications, evaluates alert rules, and dispatches notifications through aranotify when conditions are met.

## How It Works

araalert runs as a daemon with three main components:

1. **Health Checker**: On a cron schedule, runs `docker compose ps` against each deployed app to determine container health status. Results are evaluated against alert rules.
2. **Alert Manager**: Evaluates rules against current health status. When a rule fires, it creates an alert event and checks the cooldown period to avoid duplicate notifications.
3. **Dispatcher**: Sends triggered alerts to aranotify for delivery. Handles retry and event history persistence.

araalert also accepts **push events** from other tools (arabackup sends `backup-failed`, aradeploy sends `update-failed`) via its REST API.

## Commands

```
araalert run                    # Start daemon (scheduler + API)
araalert rules                  # List configured alert rules
araalert history [--limit N]    # Show alert event history
araalert test                   # Test alert rules
```

## Configuration

File: `/etc/arastack/config/araalert.yaml`

```yaml
server:
  port: 7150
  bind: 127.0.0.1

aranotify:
  url: http://127.0.0.1:7140

aradeploy:
  config: /etc/arastack/config/aradeploy.yaml

health:
  apps_dir: /opt/aradeploy/apps
  compose_cmd: docker compose
  schedule: "*/5 * * * *"      # check every 5 minutes

cooldown: 15m                   # minimum time before re-alerting
data_dir: /var/lib/araalert
```

Environment variable overrides use the `ARAALERT_` prefix.

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check with version info |
| `/api/app-health` | GET | Latest health check results for all deployed apps |
| `/api/events` | POST | Push events (used by arabackup, aradeploy) |
| `/api/rules` | GET | List configured alert rules |
| `/api/history` | GET | Query alert history (optional `?limit=N`) |

## Global Flags

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Debug logging |
| `-q`, `--quiet` | Suppress non-essential output |

## Interactions with Other Tools

- **aranotify**: Sends notifications when alert rules fire. Uses the notify client from `pkg/clients` with retry support.
- **aradeploy**: Reads aradeploy config to locate deployed apps. Runs `docker compose ps` to check container health.
- **arabackup**: Receives `backup-failed` push events via the `/api/events` endpoint.
- **aradeploy**: Receives `update-failed` push events via the `/api/events` endpoint.
- **aradashboard**: Queries `/api/app-health` for app health status, and `/api/rules` and `/api/history` to display alert information.
