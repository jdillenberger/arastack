# arascanner

Peer discovery and peer group management daemon. arascanner uses mDNS to automatically discover other arastack instances on the local network and maintains a peer registry with heartbeat-based health tracking.

## How It Works

arascanner operates as a daemon with four main subsystems:

1. **mDNS Advertiser**: Publishes this node as a `_arascanner._tcp` service via mDNS, making it discoverable by other arastack peers.
2. **mDNS Discoverer**: Periodically scans the local network for other `_arascanner._tcp` services and adds discovered peers to the store.
3. **Heartbeat Loop**: Periodically pings known peers via their HTTP API to track online/offline status based on response time.
4. **API Server**: Exposes a REST API for querying peers, handling join requests, and responding to heartbeats.

Peers are organized into **peer groups**. A peer group has a name and shared secret. New nodes can join an existing peer group using an invite token (displayed as a QR code).

## Commands

```
arascanner run                  # Start daemon
arascanner peers                # List known peers
arascanner invite               # Generate join token (+ QR code)
arascanner join <token>         # Join an existing peer group
arascanner tags                 # Manage peer tags
```

## Configuration

arascanner uses CLI flags and environment variables (no config file):

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `ARASCANNER_PORT` | 7120 | API listen port |
| `--data-dir` | `ARASCANNER_DATA_DIR` | `/var/lib/arascanner` | State directory |
| `--hostname` | `ARASCANNER_HOSTNAME` | System hostname | Node display name |
| `--discovery-interval` | `ARASCANNER_DISCOVERY_INTERVAL` | — | mDNS scan interval |
| `--heartbeat-interval` | `ARASCANNER_HEARTBEAT_INTERVAL` | — | Peer ping interval |
| `--offline-threshold` | `ARASCANNER_OFFLINE_THRESHOLD` | — | Time before marking peer offline |

## State

Peer group and peer data is persisted as YAML in the data directory:

```yaml
peer_group:
  name: homelab
  secret: <shared-secret>
peers:
  - hostname: server1
    address: 192.168.1.10
    port: 7120
    version: 1.2.3
    role: member
    tags: [docker, backup]
    online: true
    last_seen: 2025-03-09T12:00:00Z
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check with version info |
| `/api/peers` | GET | Peer group info and peer list |
| `/api/join` | POST | Remote peer join request |
| `/api/heartbeat` | POST | Peer ping/pong |

## Global Flags

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Debug logging |

## Interactions with Other Tools

- **aradashboard**: Queries `/api/peers` to display the peer group overview (peer list, online/offline status, versions).
- **aramanager**: Manages arascanner's systemd service and runs its doctor checks.
- **Network**: Uses mDNS (multicast DNS) on local network interfaces for zero-configuration peer discovery. Detects the local IP via `pkg/netutil`.
