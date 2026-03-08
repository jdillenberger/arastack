# aramanager

Centralized management tool for the AraStack suite. Provides version tracking, updates, diagnostics, systemd service management, and initial setup.

## Commands

| Command | Description |
|---------|-------------|
| `aramanager setup` | Interactive setup wizard for the entire suite |
| `aramanager version` | Show version information for all installed arastack tools |
| `aramanager update` | Update arastack tools to the latest release |
| `aramanager doctor` | Diagnose system configuration issues |
| `aramanager service` | Manage systemd services (start, stop, enable, disable) |
| `aramanager config` | Configuration management for all tools |
| `aramanager uninstall` | Uninstall the arastack suite |

## Installation

`aramanager` is the bootstrap entry point. Install it via the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/jdillenberger/arastack/main/install.sh | sudo bash
```

Then run `sudo aramanager setup` to install and configure the remaining tools.

## Interactions with Other Tools

`aramanager` is the management layer for the entire suite:

- **All tools** - downloads binaries, manages updates via GitHub releases, and handles systemd service install/uninstall/start/stop/restart.
- **araalert, aranotify, arascanner, aradashboard, aramdns** - installs and manages their systemd services (`aramanager service` subcommands).
- **aradeploy, arabackup** - runs doctor checks to validate their config files exist at `/etc/arastack/config/`, creates default configs via `aramanager config init`, and manages their systemd services.
- **aramdns, arascanner** - managed via systemd only; these tools use CLI flags/env vars instead of YAML config files, so `aramanager config show/init` does not apply to them.
