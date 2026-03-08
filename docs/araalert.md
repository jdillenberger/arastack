# araalert

Alert rule evaluation daemon. Monitors deployed app health on a cron schedule, evaluates alert rules, and dispatches notifications via aranotify.

## Commands

| Command | Description |
|---------|-------------|
| `araalert run` | Run the alert evaluation daemon |
| `araalert rules list` | List configured alert rules |
| `araalert rules add` | Add an alert rule (types: `app-down`, `backup-failed`, `update-failed`) |
| `araalert rules remove <id>` | Remove an alert rule |

## Configuration

Default config path: `/etc/arastack/config/araalert.yaml`

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `7150` | API server port |
| `server.bind` | `127.0.0.1` | Bind address |
| `aranotify.url` | `http://127.0.0.1:7140` | aranotify service URL |
| `aradeploy.config` | - | Path to aradeploy config file |
| `health.apps_dir` | - | Apps directory for health checks |
| `health.compose_cmd` | `docker compose` | Docker Compose command |
| `health.schedule` | `*/5 * * * *` | Cron schedule for health checks |
| `cooldown` | `15m` | Cooldown between repeated notifications |
| `data_dir` | `/var/lib/araalert` | Data directory for persistent state |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/rules` | Get all alert rules |
| `POST` | `/api/rules` | Create a new rule |
| `DELETE` | `/api/rules/{id}` | Delete a rule |
| `GET` | `/api/history` | Get alert history |
| `POST` | `/api/events` | Post events |

## How It Works

1. A cron scheduler triggers health checks at the configured interval (default: every 5 minutes).
2. Health check results are evaluated against configured alert rules.
3. When a rule triggers, a notification is dispatched to aranotify.
4. A cooldown period prevents duplicate notifications for the same issue.
5. Alert history is persisted to the data directory.

## Interactions with Other Tools

- **aranotify** - sends alert notifications via the aranotify `/api/send` endpoint. Configured via `aranotify.url`.
- **aradeploy** - reads the aradeploy configuration to discover deployed apps and check their container health status.
- **aradashboard** - exposes alert rules and history via its REST API, which aradashboard queries for display.
- **Events API** - external tools can post events to the `/api/events` endpoint to trigger alerts (e.g. for backup or deployment failures).
