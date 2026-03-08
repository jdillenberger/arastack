# arabackup

Backup management tool for applications deployed via aradeploy. Supports borg filesystem backups and database dumps (PostgreSQL, MySQL, SQLite, MongoDB).

## Commands

| Command | Description |
|---------|-------------|
| `arabackup daemon` | Run as a daemon with scheduled backups and prunes |
| `arabackup backup [app]` | Create backup (`--type`: `all`, `borg`, `dump`) |
| `arabackup list [app]` | List backup archives |
| `arabackup restore <app> <archive>` | Restore from a backup |
| `arabackup prune [app]` | Prune old backups per retention policy |
| `arabackup status [app]` | Show backup status |
| `arabackup config` | Configuration management |

## Configuration

Default config path: `/etc/arastack/config/arabackup.yaml`

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

| Key | Description |
|-----|-------------|
| `aradeploy.config` | Path to aradeploy config for app discovery |

## How It Works

1. Reads aradeploy's app directory and docker-compose files to discover services with backup labels.
2. For borg backups: creates/initializes borg repositories and archives app data directories.
3. For database dumps: connects to database containers and exports dumps (supports PostgreSQL, MySQL, SQLite, MongoDB).
4. Retention policies automatically prune old archives.
5. Per-service label overrides in docker-compose files can customize retention settings.

## Interactions with Other Tools

- **aradeploy** - reads the aradeploy apps directory and docker-compose files to discover which apps to back up. Backup behavior is driven by labels on compose services.
- **araalert** - can post events to araalert's `/api/events` endpoint to report backup failures.
- **aradashboard** - exposes backup status via its REST API, which aradashboard queries for the backups page.
