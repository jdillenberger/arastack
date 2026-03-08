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
        +-----+------+    +------+------+    | (peer discov.) |
     REST API |                  |           +----------------+
        +-----v------+          |
        |  aranotify |          | reads config & apps dir
        |   (:7140)  |          |
        | (channels) |          |
        +------------+          |
                          +-----v------+
  reads config & ---------+  aradeploy |
  apps dir                | (deployer) |
                          +-----+------+
                                |
                          +-----v------+
                          |   aramdns  |  watches containers
                          | (mDNS pub) |
                          +------------+
```

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

Most tools use a layered YAML configuration system (aramdns and arascanner use CLI flags and environment variables instead):

1. System-wide: `/etc/arastack/config/{tool}.yaml`
2. User-level: `~/.arastack/config/{tool}.yaml`
3. Environment variables: `{TOOL}_{SECTION}_{KEY}`
4. CLI flag: `--config /path/to/config.yaml`

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
