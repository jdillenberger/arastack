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
| `aradeploy.config` | `/etc/arastack/config/aradeploy.yaml` | Path to aradeploy config file (used to resolve apps directory) |
| `health.apps_dir` | - | Apps directory override for health checks (if empty, resolved from `aradeploy.config`) |
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

araalert uses two different evaluation models depending on the rule type:

**Periodic health checks** (`app-down` rules):
1. A cron scheduler triggers health checks at the configured interval (default: every 5 minutes).
2. Health check results are evaluated against configured `app-down` alert rules.
3. When a rule triggers, a notification is dispatched to aranotify.

**Event-driven** (`backup-failed`, `update-failed` rules):
1. External tools push events to the `/api/events` endpoint (arabackup pushes `backup-failed`, aradeploy pushes `update-failed`).
2. Incoming events are matched against configured rules of the corresponding type.
3. Matching rules dispatch notifications to aranotify.

Both models share:
- A cooldown period that prevents duplicate notifications for the same issue.
- Alert history persisted to the data directory.
- Retry with exponential backoff on notification delivery failures.

**Self-monitoring:** araalert periodically checks aranotify reachability alongside its health checks. If aranotify is unreachable, a warning is logged so operators can detect notification outages via journal/logs.

## Interactions with Other Tools

- **aranotify** - sends alert notifications via the aranotify `/api/send` endpoint. Configured via `aranotify.url`.
- **aradeploy** - reads the aradeploy configuration to discover deployed apps and check their container health status (`app-down` rules). Also receives `update-failed` events from aradeploy when container updates fail.
- **arabackup** - receives `backup-failed` events from arabackup when scheduled backups fail.
- **aradashboard** - exposes alert rules and history via its REST API, which aradashboard queries for display.
