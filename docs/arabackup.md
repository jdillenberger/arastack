# arabackup

Backup management tool for applications deployed via aradeploy. Supports borg filesystem backups and database dumps (PostgreSQL, MySQL, SQLite, MongoDB).

## Commands

| Command | Description |
|---------|-------------|
| `arabackup run` | Run as a daemon with scheduled backups and prunes |
| `arabackup backup [app]` | Create backup (`--type`: `all`, `borg`, `dump`) |
| `arabackup list [app]` | List backup archives |
| `arabackup restore <app> <archive>` | Restore from a backup |
| `arabackup prune [app]` | Prune old backups per retention policy |
| `arabackup status [app]` | Show backup status |
| `arabackup config` | Configuration management |

## Configuration

Default config path: `/etc/arastack/config/arabackup.yaml`

### Server

| Key | Default | Description |
|-----|---------|-------------|
| `server.bind` | `127.0.0.1` | Bind address for the API server |
| `server.port` | `7160` | Port for the API server |

### Borg Settings

| Key | Default | Description |
|-----|---------|-------------|
| `borg.base_dir` | `/mnt/backup/borg` | Base directory for borg repositories |
| `borg.passphrase_file` | `/etc/arastack/borg-passphrase` | Path to borg passphrase |
| `borg.encryption` | `repokey` | Encryption method |

### Retention Policy

| Key | Default | Description |
|-----|---------|-------------|
| `borg.retention.keep_daily` | `7` | Daily backups to keep |
| `borg.retention.keep_weekly` | `4` | Weekly backups to keep |
| `borg.retention.keep_monthly` | `6` | Monthly backups to keep |

### Dumps

| Key | Default | Description |
|-----|---------|-------------|
| `dumps.dir` | `/opt/arabackup/dumps` | Directory for database dumps |

### Schedule (daemon mode)

| Key | Default | Description |
|-----|---------|-------------|
| `schedule.backup` | `0 3 * * *` | Cron schedule for backups (3 AM daily) |
| `schedule.prune` | `0 5 * * 0` | Cron schedule for prune (5 AM Sunday) |

### Integration

| Key | Default | Description |
|-----|---------|-------------|
| `aradeploy.config` | `/etc/arastack/config/aradeploy.yaml` | Path to aradeploy config for app discovery |
| `araalert.url` | `http://127.0.0.1:7150` | araalert URL for pushing backup-failed events |

## API Endpoints

Available when running in daemon mode (`arabackup run`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check (version, uptime) |
| `GET` | `/api/status` | Backup status (schedule, app count, last/next run) |

## Docker Compose Labels

Backup behavior is configured per-service via Docker Compose labels. Add these to the `labels` section of services in your `docker-compose.yml`:

| Label | Required | Description |
|-------|----------|-------------|
| `arabackup.enable` | Yes | Set to `true` to enable backups for this service |
| `arabackup.borg.paths` | No | Comma-separated paths relative to data_dir to back up |
| `arabackup.dump.driver` | No | Database dump driver: `postgres`, `mysql`, `mongodb`, `sqlite`, `custom` |
| `arabackup.dump.user` | No | Database user for dump authentication |
| `arabackup.dump.password-env` | No | Environment variable name containing the database password |
| `arabackup.dump.database` | No | Database name to dump |
| `arabackup.dump.command` | No | Custom dump command (for `custom` driver) |
| `arabackup.dump.restore-command` | No | Custom restore command (for `custom` driver) |
| `arabackup.dump.file-ext` | No | File extension for custom dump output |
| `arabackup.retention.keep-daily` | No | Override daily retention (default: 7) |
| `arabackup.retention.keep-weekly` | No | Override weekly retention (default: 4) |
| `arabackup.retention.keep-monthly` | No | Override monthly retention (default: 6) |

### Example

```yaml
services:
  postgres:
    image: postgres:16
    labels:
      arabackup.enable: "true"
      arabackup.dump.driver: postgres
      arabackup.dump.user: myapp
      arabackup.dump.password-env: POSTGRES_PASSWORD
      arabackup.dump.database: myapp
      arabackup.retention.keep-daily: "14"
```

## How It Works

1. Reads aradeploy's app directory and docker-compose files to discover services with backup labels.
2. For borg backups: creates/initializes borg repositories and archives app data directories.
3. For database dumps: connects to database containers and exports dumps (supports PostgreSQL, MySQL, SQLite, MongoDB).
4. Retention policies automatically prune old archives.
5. Per-service label overrides in docker-compose files can customize retention settings.

## Interactions with Other Tools

- **aradeploy** - reads the aradeploy apps directory and docker-compose files to discover which apps to back up. Backup behavior is driven by labels on compose services.
- **araalert** - pushes `backup-failed` events to araalert's `/api/events` endpoint when a backup fails (with retry on failure). araalert matches these against configured alert rules and dispatches notifications via aranotify.
- **aradashboard** - exposes backup status via its REST API, which aradashboard queries for the backups page.
