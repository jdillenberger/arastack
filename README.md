# AraStack

A modular homelab infrastructure toolkit written in Go. AraStack provides a suite of autonomous CLI tools for deploying, monitoring, alerting, backing up, and discovering services across a homelab fleet.

## Architecture

AraStack consists of 8 tools that communicate via REST APIs:

```
                        +-----------------+
                        |  aradashboard   |  Web UI (:8420)
                        |  (monitoring)   |
                        +---+----+----+---+
                            |    |    |
              +-------------+    |    +--------------+
              |                  |                   |
        +-----v-----+    +------v------+    +--------v-------+
        |  araalert  |    |  arabackup  |    |   arascanner   |
        |   (:7150)  |    |  (backup)   |    |    (:7120)     |
        +-----+------+    +------+------+    | (peer discov.) |
              |                  |           +----------------+
        +-----v------+    +-----v------+
        |  aranotify |    |  aradeploy |
        |   (:7140)  |    | (deployer) |
        | (channels) |    +-----+------+
        +------------+          |
                          +-----v------+    +------------+
                          |   aramdns  |    | aramanager |
                          | (mDNS pub) |    |  (setup)   |
                          +------------+    +------------+
```

## Tools

| Tool | Type | Default Port | Description | Docs |
|------|------|-------------|-------------|------|
| [aramanager](docs/aramanager.md) | CLI | - | Centralized setup, updates, and service management | [docs](docs/aramanager.md) |
| [aradeploy](docs/aradeploy.md) | CLI | - | Template-based Docker Compose deployment | [docs](docs/aradeploy.md) |
| [aradashboard](docs/aradashboard.md) | Daemon | 8420 | Web dashboard for monitoring | [docs](docs/aradashboard.md) |
| [araalert](docs/araalert.md) | Daemon | 7150 | Health check evaluation and alert dispatching | [docs](docs/araalert.md) |
| [aranotify](docs/aranotify.md) | Daemon | 7140 | Multi-channel notification delivery | [docs](docs/aranotify.md) |
| [arabackup](docs/arabackup.md) | CLI/Daemon | - | Borg backup and database dump management | [docs](docs/arabackup.md) |
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

All tools use a layered YAML configuration system:

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
  clients/              # HTTP API clients for inter-app communication
  config/               # Unified YAML config framework
  health/               # Health check utilities
  systemd/              # Systemd service management
  version/              # Version/build metadata
```

## License

Private.
