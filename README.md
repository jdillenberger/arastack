# AraStack

A modular homelab infrastructure toolkit written in Go. AraStack provides a suite of autonomous CLI tools for deploying, monitoring, alerting, backing up, and discovering services across a homelab fleet.

## Architecture

AraStack consists of 8 tools that integrate via a mix of REST APIs (aradashboard querying arascanner/araalert/arabackup) and filesystem-based integration (tools reading aradeploy's config and app directories):

```
  +------------+         manages all tools
  | aramanager |------------------------------------+
  |  (setup)   |                                    |
  +------------+                                    |
                                                    v
                        +-----------------+    installs &
                        |  aradashboard   |    configures
                        |  Web UI (:8420) |
                        +---+----+----+---+
                 REST API   |    |    |   REST API
              +-------------+    |    +--------------+
              |                  | REST API          |
        +-----v-----+    +------v------+    +--------v-------+
        |  araalert  |    |  arabackup  |    |   arascanner   |
        |   (:7150)  |    |   (:7160)   |    |    (:7120)     |
        +--+---+--+--+    +---+----+----+    | (peer discov.) |
   REST    |   |  ^ events    |    |         +----------------+
   API     |   |  | (push)    |    |
     +-----v-+ |  |           |    | reads config
     |aranotif| |  +-----------+   | & apps dir
     | (:7140)| |  |               |
     +--------+ |  | events (push) |
                 |  |               |
        reads    |  +----+-----+----+
        config   +------>|           |
        & apps dir       | aradeploy |
                         | (deployer)|
                         +-----+-----+
                               |
                         +-----v------+
                         |   aramdns  |  watches containers
                         | (mDNS pub) |
                         +------------+
```

**Integration types:**
- **REST API queries**: aradashboard → arascanner, araalert, arabackup
- **REST API events (push)**: arabackup → araalert (`backup-failed`), aradeploy → araalert (`update-failed`)
- **REST API notifications**: araalert → aranotify
- **Filesystem (config + app dirs)**: araalert, arabackup, aradashboard all read aradeploy's config and app directories

## Tools

| Tool | Type | Default Port | Description | Docs |
|------|------|-------------|-------------|------|
| [aramanager](docs/aramanager.md) | CLI | - | Centralized setup, updates, and service management | [docs](docs/aramanager.md) |
| [aradeploy](docs/aradeploy.md) | CLI | - | Template-based Docker Compose deployment | [docs](docs/aradeploy.md) |
| [aradashboard](docs/aradashboard.md) | Daemon | 8420 | Web dashboard for monitoring | [docs](docs/aradashboard.md) |
| [araalert](docs/araalert.md) | Daemon | 7150 | Health check evaluation and alert dispatching | [docs](docs/araalert.md) |
| [aranotify](docs/aranotify.md) | Daemon | 7140 | Multi-channel notification delivery | [docs](docs/aranotify.md) |
| [arabackup](docs/arabackup.md) | CLI/Daemon | 7160 | Borg backup and database dump management | [docs](docs/arabackup.md) |
| [arascanner](docs/arascanner.md) | Daemon | 7120 | mDNS-based fleet peer discovery | [docs](docs/arascanner.md) |
| [aramdns](docs/aramdns.md) | Daemon | - | Publishes Traefik .local domains via Avahi mDNS | [docs](docs/aramdns.md) |

> **Note on mDNS:** aramdns and arascanner both use mDNS but for different purposes and do not conflict. aramdns publishes address (A) records for `.local` domains via Avahi. arascanner advertises and discovers `_arascanner._tcp` service records via zeroconf. They operate on different mDNS record types.

## Quick Start

Install `aramanager` (the bootstrap tool):

```bash
curl -fsSL https://raw.githubusercontent.com/jdillenberger/arastack/main/install.sh | sudo bash
```

Then run the setup wizard:

```bash
sudo aramanager setup
```

## Building from Source

Requires Go 1.24+.

```bash
# Build all tools
make build

# Build a single tool
make build-araalert

# Run tests
make test

# Install to /usr/local/bin
make install

# Cross-compile for ARM64
make build-arm64

# Build snapshot release
make release
```

## Configuration

Most tools use a layered YAML configuration system:

1. System-wide: `/etc/arastack/config/{tool}.yaml`
2. User-level: `~/.arastack/config/{tool}.yaml`
3. Environment variables: `{TOOL}_{SECTION}_{KEY}`
4. CLI flag: `--config /path/to/config.yaml`

**Exception:** aramdns and arascanner use CLI flags and environment variables (`ARAMDNS_*`, `ARASCANNER_*`) instead of YAML config files. These tools have very few settings (2 and 6 respectively) and are configured via their systemd unit environment. `aramanager config show/init` does not apply to them.

## API Authentication

arascanner's API is protected by a Pre-Shared Key (PSK). The other service APIs (araalert, aranotify, arabackup) bind to `127.0.0.1` by default and are only accessible locally. aradashboard binds to `0.0.0.0` by default to serve its web UI — use a reverse proxy or firewall to restrict access in untrusted networks.

## Supported Platforms

- Linux amd64
- Linux arm64
- Linux armv7

## Project Structure

```
cmd/                    # Entry points for all 8 tools
internal/               # App-specific logic (cli, api, config, etc.)
pkg/                    # Shared libraries
  aradeployconfig/      # Aradeploy config loader
  clients/              # HTTP API clients for inter-app communication
  config/               # Unified YAML config framework
  doctor/               # System diagnostics utilities
  executil/             # Shell command execution helpers
  health/               # Health check utilities
  netutil/              # Network utility functions
  selfupdate/           # Binary self-update mechanism
  systemd/              # Systemd service management
  version/              # Version/build metadata
```

## License

Private.
