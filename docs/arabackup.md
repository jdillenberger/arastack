# arabackup

Scheduled backup service for applications deployed by aradeploy. arabackup combines Borg for filesystem backups with pluggable database dump drivers, automatic retention management, and alert integration.

## How It Works

arabackup discovers applications by scanning aradeploy's app directories for docker-compose files with `arabackup.*` labels. For each enabled service, it:

1. **Discovers** apps with `arabackup.enable=true` in their docker-compose labels
2. **Dumps** databases by running driver-specific commands inside Docker containers (e.g., `pg_dump` via `docker exec`)
3. **Archives** using Borg — creates compressed, deduplicated archives containing the app's data directory and any database dump files
4. **Prunes** old archives based on configurable retention policies (daily/weekly/monthly)

In daemon mode (`arabackup run`), it schedules backup and prune jobs via cron expressions, runs a REST API for status queries, and pushes failure events to araalert. Failed event deliveries are spooled to disk and retried on the next backup cycle.

## Commands

```
arabackup backup [app]          # Create backups (all apps or specific)
  --type borg                   # Borg archive only
  --type dump                   # Database dumps only

arabackup restore <app> [archive]  # Restore from backup
  --yes                            # Skip confirmation

arabackup prune [app]           # Apply retention policy
arabackup list [app]            # List borg archives
arabackup status                # Show backup status for all apps

arabackup run                   # Run as daemon (scheduled backups)
arabackup daemon                # Alias for run

arabackup config show           # Display effective config
arabackup config init           # Generate default config
```

### Restore Flow

1. Shows interactive confirmation dialog (unless `--yes`)
2. Stops the app via `docker compose down`
3. Extracts Borg archive (filesystem restore)
4. Starts only database services
5. Waits 10 seconds for databases to initialize
6. Restores database dumps
7. Stops database services
8. Starts the full app

**Warning:** Restore does not have rollback. If a step fails partway through (e.g., borg extraction succeeds but the database restore fails), your app may be left in an inconsistent state. Consider creating a fresh backup before restoring from an older archive.

## Configuration

File: `/etc/arastack/config/arabackup.yaml`

```yaml
server:
  bind: 127.0.0.1
  port: 7160

borg:
  base_dir: /mnt/backup/borg
  passphrase_file: /etc/arastack/borg-passphrase
  encryption: repokey
  retention:
    keep_daily: 7
    keep_weekly: 4
    keep_monthly: 6

dumps:
  dir: /opt/arabackup/dumps

schedule:
  backup: "0 3 * * *"         # daily at 3am
  prune: "0 5 * * 0"          # weekly on Sunday at 5am

aradeploy:
  config: /etc/arastack/config/aradeploy.yaml

araalert:
  url: http://127.0.0.1:7150
```

Environment variable overrides use the `ARABACKUP_` prefix.

## Docker Compose Labels

Services opt into backups and configure dump drivers via labels:

```yaml
labels:
  arabackup.enable: "true"
  arabackup.borg.paths: "data,config"          # relative to data_dir
  arabackup.dump.driver: "postgres"            # postgres|mysql|mongodb|sqlite|custom
  arabackup.dump.user: "postgres"
  arabackup.dump.password-env: "POSTGRES_PASSWORD"
  arabackup.dump.database: "mydb"              # or "all"
  arabackup.dump.command: "my-dump-script"     # custom driver only
  arabackup.dump.restore-command: "my-restore" # custom driver only
  arabackup.dump.file-ext: "bak"               # custom driver only
  arabackup.retention.keep-daily: "14"         # override global retention
  arabackup.retention.keep-weekly: "8"
  arabackup.retention.keep-monthly: "12"
```

## Database Dump Drivers

| Driver | Dump Command | Restore Command |
|--------|-------------|-----------------|
| `postgres` | `pg_dump` / `pg_dumpall` | `psql` |
| `mysql` | `mysqldump` | `mysql` |
| `mongodb` | `mongodump` | `mongorestore` |
| `sqlite` | `sqlite3 .dump` | `sqlite3` |
| `custom` | User-specified command | User-specified command |

All dump/restore commands run inside the service's Docker container via `docker exec`.

## API Endpoints (Daemon Mode)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check with version info |
| `/api/status` | GET | Backup status: schedule, next/last run, app count, repo info |

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path |
| `-v`, `--verbose` | Debug logging |
| `-q`, `--quiet` | Suppress non-essential output |
| `--json` | JSON output (status/config) |

## Interactions with Other Tools

- **aradeploy**: Reads aradeploy's config (`apps_dir`, `data_dir`, `docker.compose_command`) and scans app directories for docker-compose files. Backup labels on services control what gets backed up and how.
- **araalert**: Pushes `backup-failed` events on failures. Events are spooled to `/var/lib/arabackup/pending-events.json` if araalert is unreachable and retried at the start of each backup cycle.
- **aradashboard**: Queries arabackup's `/api/status` endpoint to display backup status in the dashboard.
- **Docker**: Uses `docker exec` for database dumps/restores and `docker compose` for stopping/starting apps during restore.
- **Borg**: Direct CLI wrapper — initializes repos, creates/extracts/prunes archives, manages encryption via `BORG_REPO` and `BORG_PASSPHRASE` environment variables.
