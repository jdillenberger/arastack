# aramanager

Unified management tool for the arastack suite. aramanager handles installation, updates, health checks, systemd service management, and complete setup/teardown of all arastack tools from a single CLI.

## How It Works

aramanager maintains an internal registry of all arastack tools in dependency order. When running `setup`, it processes tools sequentially — arascanner first, aramdns last — ensuring services that depend on others are started after their dependencies.

Each tool in the registry exposes Go-level `DoctorCheck()` and `DoctorFix()` functions that aramanager calls directly (not as subprocesses). This tight integration means aramanager can validate the entire stack's health in one pass and auto-fix issues like missing system packages.

For updates, aramanager fetches the latest GitHub release, downloads a combined `arastack_{os}_{arch}.tar.gz` archive, extracts each binary, and atomically replaces the installed versions.

## Managed Tools

| Order | Tool | Port | Config |
|-------|------|------|--------|
| 1 | arascanner | 7120 | — |
| 2 | aranotify | 7140 | `/etc/arastack/config/aranotify.yaml` |
| 3 | araalert | 7150 | `/etc/arastack/config/araalert.yaml` |
| 4 | arabackup | 7160 | `/etc/arastack/config/arabackup.yaml` |
| 5 | aradashboard | 8420 | `/etc/arastack/config/aradashboard.yaml` |
| 6 | aradeploy | — | `/etc/arastack/config/aradeploy.yaml` |
| 7 | aramdns | — | — |

## Commands

### `aramanager setup [--skip tool1,tool2]`

Full setup of all tools:

1. Checks for missing binaries in `$PATH`
2. Downloads missing binaries from the latest GitHub release
3. For each tool (in dependency order):
   - Runs doctor checks
   - Auto-fixes non-optional failures (e.g., installs missing apt packages)
   - Creates and starts the systemd service

Use `--skip` to skip specific tools (comma-separated names).

### `aramanager update [--check]`

Fetches the latest release from `github.com/jdillenberger/arastack` and updates all installed binaries. With `--check`, only reports whether updates are available without installing.

### `aramanager doctor [tool] [--fix]`

Runs health checks across all tools (or a specific one). Shows installed status, versions, and dependency results. With `--fix`, attempts to install missing system dependencies via `apt`.

### `aramanager service <action> [tool]`

Systemd service management:

| Action | Description |
|--------|-------------|
| `install [tool]` | Write unit file, enable and start service |
| `uninstall [tool]` | Stop, disable, and remove service |
| `start [tool]` | Start service |
| `stop [tool]` | Stop service |
| `restart [tool]` | Restart service |
| `status [tool]` | Show service status (all tools if none specified) |

### `aramanager config show [tool]`

Displays configuration file paths and contents for each tool.

### `aramanager config init [tool]`

Creates default configuration files by running doctor checks with auto-fix.

### `aramanager uninstall [--yes]`

Interactive uninstall wizard that optionally removes:
- All systemd services and binaries
- Configuration files (`/etc/arastack/config/`, `~/.arastack/config/`)
- Deployed applications (`/opt/aradeploy/apps`)
- aramanager itself

Use `--yes` to skip the interactive prompts and remove everything.

### `aramanager version`

Prints version, commit hash, and build date.

## Global Flags

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Enable debug logging |

## Interactions with Other Tools

- **All tools**: aramanager imports each tool's `doctor` package to run health checks and fixes directly in-process.
- **GitHub**: Fetches release metadata and downloads binary archives from `github.com/jdillenberger/arastack`.
- **systemd**: Generates unit files, manages service lifecycle via `systemctl`.
- **apt**: Doctor fix mode can install missing system packages via `sudo apt install`.
