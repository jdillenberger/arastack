# arastack

A self-hosted homelab management suite. arastack provides deployment, backup, monitoring, alerting, notifications, peer discovery, and a web dashboard — all managed through a unified CLI.

## Tools

| Tool | Description | Port | Docs |
|------|-------------|------|------|
| [aramanager](docs/aramanager.md) | Unified manager: install, update, and orchestrate all tools | — | [docs](docs/aramanager.md) |
| [aradeploy](docs/aradeploy.md) | Deploy Docker Compose apps from templates | — | [docs](docs/aradeploy.md) |
| [arabackup](docs/arabackup.md) | Scheduled backups via Borg + database dumps | 7160 | [docs](docs/arabackup.md) |
| [araalert](docs/araalert.md) | Health monitoring and alert rule evaluation | 7150 | [docs](docs/araalert.md) |
| [aranotify](docs/aranotify.md) | Multi-channel notification delivery | 7140 | [docs](docs/aranotify.md) |
| [arascanner](docs/arascanner.md) | Peer discovery and fleet management via mDNS | 7120 | [docs](docs/arascanner.md) |
| [aramdns](docs/aramdns.md) | Publish Traefik domains via mDNS/Avahi | — | [docs](docs/aramdns.md) |
| [aradashboard](docs/aradashboard.md) | Web dashboard aggregating all services | 8420 | [docs](docs/aradashboard.md) |

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        aramanager                             │
│         (setup, update, doctor, systemd orchestration)        │
└─────────────────────────────┬────────────────────────────────┘
                              │ manages
      ┌──────────┬────────────┼──────────┬──────────┐
      ▼          ▼            ▼          ▼          ▼
 arascanner  aranotify    araalert   arabackup  aradashboard
   :7120       :7140        :7150      :7160       :8420
                 ▲            ▲                      │
                 │            │ push events           │
                 │            ├── arabackup           │
                 │            │   (backup-failed)     │
                 │            └── aradeploy           │
                 │                (update-failed)     │
                 │                                    │
                 └── araalert                         │
                     (sends alerts)                   │
                                                      │
                 queries ◄────────────────────────────┘
                 arascanner, arabackup, araalert APIs

  aradeploy (deploys Docker Compose apps)
      │  arabackup, araalert, aradashboard read its app dirs
      │
      └── aramdns (watches containers, publishes Traefik domains via mDNS)
```

### How the tools interact

- **aramanager** downloads, installs, and manages systemd services for all other tools. It runs each tool's doctor checks and can auto-fix missing dependencies.
- **aradeploy** deploys Docker Compose applications from templates. Other tools discover deployed apps by reading aradeploy's configuration and scanning its apps directory.
- **arabackup** discovers apps deployed by aradeploy (via docker-compose labels), creates Borg archives and database dumps, and pushes failure events to araalert.
- **araalert** periodically checks the health of deployed apps (container status) and evaluates alert rules. When a rule fires, it sends notifications through aranotify.
- **aranotify** delivers notifications to configured channels: webhooks, ntfy, email (SMTP), and Mattermost.
- **arascanner** discovers other arastack peers on the local network via mDNS and maintains a fleet registry with heartbeat-based online/offline tracking.
- **aramdns** watches running Docker containers for Traefik routing labels and publishes their domains via Avahi mDNS, making them resolvable on the local network.
- **aradashboard** provides a web UI that aggregates data from arascanner (peers), arabackup (backup status), araalert (alert history), and aradeploy (deployed apps).

### Shared packages

All tools share common functionality via `pkg/`:

| Package | Purpose |
|---------|---------|
| `pkg/config` | Layered configuration loading (YAML file + env overrides) |
| `pkg/executil` | Command execution wrapper with env, streaming, and piping support |
| `pkg/clients` | HTTP API clients for inter-tool communication (alert, notify, backup, scanner) |
| `pkg/systemd` | systemd unit file generation and service lifecycle management |
| `pkg/version` | Build-time version info (set via ldflags) |
| `pkg/health` | Standardized `/api/health` endpoint |
| `pkg/doctor` | System dependency checking and auto-fix (apt install) |
| `pkg/selfupdate` | Binary extraction from tar.gz and atomic replacement |
| `pkg/netutil` | Local IP detection |
| `pkg/aradeployconfig` | Shared aradeploy config types so other tools can read deployment state |

## Quick Start

### Install

```bash
curl -fsSL https://raw.githubusercontent.com/jdillenberger/arastack/main/install.sh | sudo bash
```

This installs `aramanager`. Then set up all tools:

```bash
sudo aramanager setup
```

The setup command downloads all tool binaries, runs doctor checks with auto-fix, and installs systemd services in the correct dependency order.

### Build from source

```bash
git clone https://github.com/jdillenberger/arastack.git
cd arastack
make build        # builds all tools to bin/
sudo make install # installs to /usr/local/bin
```

### Update

```bash
sudo aramanager update          # update all tools
sudo aramanager update --check  # check for updates without installing
```

## Configuration

All configuration files live under `/etc/arastack/config/`:

| File | Tool |
|------|------|
| `aradeploy.yaml` | aradeploy |
| `arabackup.yaml` | arabackup |
| `araalert.yaml` | araalert |
| `aranotify.yaml` | aranotify |
| `aradashboard.yaml` | aradashboard |

All tools support environment variable overrides with the pattern `TOOLNAME_SECTION_KEY` (e.g., `ARABACKUP_BORG_BASE_DIR=/custom/path`).

## File Locations

| Path | Purpose |
|------|---------|
| `/etc/arastack/config/` | Configuration files |
| `/opt/aradeploy/apps/` | Deployed application directories |
| `/opt/aradeploy/data/` | Application data volumes |
| `/mnt/backup/borg/` | Borg backup repositories |
| `/var/lib/arascanner/` | arascanner peer state |
| `/var/lib/araalert/` | araalert event history |
| `~/.aradeploy/templates/` | Local template overrides |
| `~/.aradeploy/repos/` | Cloned template repositories |

## Development

```bash
make build           # build all tools
make test            # run tests
make lint            # run golangci-lint
make fmt             # format code
make vet             # run go vet
make release         # goreleaser snapshot build
make run-aradeploy ARGS="list"  # build and run a specific tool
```

## Supported Platforms

- Linux (amd64, arm64, armv7)
