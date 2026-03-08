# aramdns

mDNS publisher for Traefik-managed domains. Watches Docker containers for Traefik router labels and publishes `.local` domains via Avahi mDNS, enabling local network name resolution for deployed apps.

## Commands

| Command | Description |
|---------|-------------|
| `aramdns run` | Run the mDNS publisher daemon |

## Configuration

Configuration via CLI flags and environment variables:

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--runtime` | `ARAMDNS_RUNTIME` | auto-detected | Container runtime (`docker` or `podman`) |
| `--interval` | `ARAMDNS_INTERVAL` | `30s` | Poll interval for domain reconciliation |

## How It Works

1. Polls Docker containers at the configured interval.
2. Inspects containers for Traefik router labels (e.g., `` traefik.http.routers.*.rule=Host(`app.local`) ``).
3. Extracts `.local` domain names from the Traefik `Host()` rules.
4. Publishes discovered domains via Avahi mDNS.
5. Reconciles on each poll: removes stale publishers, adds new ones.
6. Cleans up stale Avahi publisher processes.
7. Ensures Avahi config uses physical interfaces only (prevents Docker bridge hijacking).

## Interactions with Other Tools

- **aradeploy** - works in conjunction with aradeploy deployments. When aradeploy deploys an app with Traefik routing labels, aramdns automatically picks up the `.local` domain and publishes it via mDNS, making the app resolvable on the local network without manual DNS configuration.
- **arascanner** - both tools use mDNS but do not conflict. aramdns publishes address (A) records via `avahi-publish`, while arascanner uses `_arascanner._tcp` service records via the zeroconf library. They operate on different mDNS record types and namespaces.
