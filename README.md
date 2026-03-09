# arastack

A self-hosted homelab management suite. Deploy Docker Compose apps from templates, get automatic backups, health monitoring, alerts, local DNS — and manage it all from a web dashboard or CLI.

```
curl -fsSL https://raw.githubusercontent.com/jdillenberger/arastack/main/install.sh | sudo bash
sudo aramanager setup
aradeploy deploy nextcloud
```

That's it. Nextcloud is running, backed up nightly, monitored, and reachable at `nextcloud.<hostname>.local` on your LAN.

## What You Get

| Feature | Tool | Description |
|---------|------|-------------|
| **App deployment** | [aradeploy](docs/aradeploy.md) | Deploy apps from [ready-made templates](https://github.com/jdillenberger/arastack-templates) with one command |
| **Reverse proxy** | Traefik (auto-managed) | HTTPS, subdomains, automatic certificate management |
| **Local DNS** | [aramdns](docs/aramdns.md) | Apps reachable via mDNS on your LAN (e.g. `appname.home.local`) |
| **Backups** | [arabackup](docs/arabackup.md) | Scheduled Borg archives + database dumps with retention |
| **Monitoring** | [araalert](docs/araalert.md) | Health checks on deployed apps every 5 minutes |
| **Notifications** | [aranotify](docs/aranotify.md) | Alerts via email, ntfy, webhooks, or Mattermost |
| **Peer discovery** | [arascanner](docs/arascanner.md) | Discover other arastack nodes on your network |
| **Web dashboard** | [aradashboard](docs/aradashboard.md) | See everything in one place at port 8420 |
| **Management** | [aramanager](docs/aramanager.md) | Install, update, and health-check the whole stack |

## Getting Started

See the **[Getting Started Guide](docs/getting-started.md)** for a complete walkthrough — from install to deploying your first app.

## How It Works

```
 You run:  aradeploy deploy nextcloud
                    │
                    ▼
        ┌───────────────────────┐
        │      aradeploy        │  Renders template → docker-compose.yml
        │  (template engine)    │  Injects Traefik labels for routing
        └───────────┬───────────┘
                    │ docker compose up -d
                    ▼
        ┌───────────────────────┐
        │   Docker containers   │  App + database running
        │   + Traefik proxy     │  HTTPS via Let's Encrypt
        └───────────┬───────────┘
                    │
       ┌────────────┼────────────┬──────────────┐
       ▼            ▼            ▼              ▼
   aramdns      arabackup    araalert     aradashboard
   publishes    backs up     monitors     shows it all
   mDNS names   data nightly health       in a web UI
```

All tools discover deployed apps automatically by reading aradeploy's state files and docker-compose labels. No extra configuration needed.

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Install arastack and deploy your first app |
| [Deploying Apps](docs/deploying-apps.md) | How deployments work, templates, data storage, routing |
| [Without arastack](docs/without-arastack.md) | Achieve the same with plain Docker Compose + Traefik |

### Tool Reference

| Tool | Docs |
|------|------|
| aramanager | [docs/aramanager.md](docs/aramanager.md) |
| aradeploy | [docs/aradeploy.md](docs/aradeploy.md) |
| arabackup | [docs/arabackup.md](docs/arabackup.md) |
| araalert | [docs/araalert.md](docs/araalert.md) |
| aranotify | [docs/aranotify.md](docs/aranotify.md) |
| arascanner | [docs/arascanner.md](docs/arascanner.md) |
| aramdns | [docs/aramdns.md](docs/aramdns.md) |
| aradashboard | [docs/aradashboard.md](docs/aradashboard.md) |

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        aramanager                            │
│         (setup, update, doctor, systemd orchestration)       │
└─────────────────────────────┬────────────────────────────────┘
                              │ manages
      ┌──────────┬────────────┼──────────┬──────────┐
      ▼          ▼            ▼          ▼          ▼
 arascanner  aranotify    araalert   arabackup  aradashboard
   :7120       :7140        :7150      :7160       :8420
                 ▲            │                      │
                 │ sends      │ checks health        │
                 │ alerts     │ via docker compose    │
                 │            ▼                       │
                 │        aradeploy ──► aramdns       │
                 │        (deploys)    (mDNS)          │
                 │                                    │
                 │    queries arascanner, arabackup,   │
                 └──── araalert APIs ◄────────────────┘
```

### Service Ports

| Tool | Port | Purpose |
|------|------|---------|
| arascanner | 7120 | Peer discovery API |
| aranotify | 7140 | Notification API |
| araalert | 7150 | Alert API |
| arabackup | 7160 | Backup status API |
| aradashboard | 8420 | Web dashboard |
| aradeploy | — | CLI tool (no daemon) |
| aramdns | — | Daemon (no API) |

### File Locations

| Path | Purpose |
|------|---------|
| `/etc/arastack/config/` | Configuration files |
| `/opt/aradeploy/apps/` | Deployed app directories (compose files, state) |
| `/opt/aradeploy/data/` | Application data volumes |
| `/mnt/backup/borg/` | Borg backup repositories |
| `/opt/arabackup/dumps/` | Database dump files |
| `/var/lib/ara*/` | Service state (peers, alerts, sessions) |
| `~/.aradeploy/templates/` | Local template overrides |
| `~/.aradeploy/repos/` | Cloned template repositories |

## Configuration

All config files live in `/etc/arastack/config/`. Each tool has its own YAML file. All support environment variable overrides with the prefix `TOOLNAME_SECTION_KEY`:

```bash
# Example: override arabackup's borg base directory
export ARABACKUP_BORG_BASE_DIR=/custom/backup/path
```

See each tool's docs for the full config reference.

## Development

```bash
git clone https://github.com/jdillenberger/arastack.git
cd arastack
make build           # build all tools to bin/
sudo make install    # install to /usr/local/bin
make test            # run tests
make lint            # run golangci-lint
```

## Supported Platforms

Linux (amd64, arm64, armv7)
