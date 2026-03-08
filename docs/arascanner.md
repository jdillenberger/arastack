# arascanner

Fleet peer discovery and tracking service. Discovers peers on the local network via mDNS and supports remote peer joining via invite tokens.

## Commands

| Command | Description |
|---------|-------------|
| `arascanner run` | Run the scanner daemon |
| `arascanner peers` | List known peers from the running daemon |
| `arascanner peers discover` | Run one-shot mDNS discovery |
| `arascanner join` | Join an existing fleet using an invite token |
| `arascanner invite` | Generate invite tokens for remote peers |
| `arascanner tags` | Manage peer tags |
| `arascanner show-secret` | Display fleet secret |

## Configuration

Configuration via CLI flags and environment variables:

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `ARASCANNER_PORT` | `7120` | API server port |
| `--data-dir` | `ARASCANNER_DATA_DIR` | `/var/lib/arascanner` | Data directory |
| `--hostname` | `ARASCANNER_HOSTNAME` | - | Hostname override |
| `--discovery-interval` | `ARASCANNER_DISCOVERY_INTERVAL` | `30s` | mDNS discovery interval |
| `--heartbeat-interval` | `ARASCANNER_HEARTBEAT_INTERVAL` | `60s` | Heartbeat interval |
| `--offline-threshold` | `ARASCANNER_OFFLINE_THRESHOLD` | `3m` | Mark peer offline after this |

## API Endpoints

All endpoints except `/api/health` require Bearer token (PSK) authentication.

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/health` | No | Health check |
| `GET` | `/api/peers` | Yes | Get all known peers |
| `GET` | `/api/peers/events` | Yes | Stream peer events |
| `POST` | `/api/join` | Token | Join fleet with invite token |
| `POST` | `/api/heartbeat` | Yes | Send heartbeat |

## Peer Metadata

Each discovered peer tracks:

- Hostname, IP address, port
- Role (compute, storage, control, etc.)
- Tags (arbitrary key=value pairs)
- Last seen timestamp and online/offline status
- Discovery source (mDNS or remote)

## Fleet Authentication

API endpoints (except `/api/health`) are protected by a Pre-Shared Key (PSK).

- **Auto-generated on first start:** When the daemon starts for the first time, a PSK is generated (32 bytes, hex-encoded) and stored in `peers.yaml` in the data directory.
- **Sharing via invite/join:** Use `arascanner invite` to generate a time-limited invite token that embeds the PSK. The new peer runs `arascanner join <token>` to join the fleet and receive the PSK.
- **Display the PSK:** Use `arascanner show-secret` to view the current fleet secret.
- **aradashboard integration:** The PSK must be configured manually in aradashboard's config under `services.peer_scanner.secret`.

## How It Works

1. Advertises itself via mDNS on the local network.
2. Runs periodic mDNS discovery (default: every 30s) to find other peers.
3. Maintains a persistent peer database on disk.
4. Tracks peer liveness via heartbeats; marks peers offline after the threshold.
5. Supports remote peers (outside the LAN) via time-limited invite tokens.
6. Gossips peer information between cluster members.

## Interactions with Other Tools

- **aradashboard** - exposes peer data via the `/api/peers` endpoint (authenticated via PSK). aradashboard displays fleet information on its fleet page.
