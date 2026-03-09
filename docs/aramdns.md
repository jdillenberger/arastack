# aramdns

Traefik domain mDNS publisher. aramdns watches running Docker containers for Traefik routing labels and publishes their domains via Avahi mDNS, making them resolvable on the local network without manual DNS configuration.

## How It Works

aramdns runs as a daemon that:

1. **Configures Avahi**: Ensures the Avahi daemon is configured to advertise only on physical network interfaces (excludes Docker bridge networks).
2. **Discovers domains**: Periodically queries Docker (or Podman) for running containers with Traefik routing labels. Extracts hostnames from `traefik.http.routers.<router>.rule` labels (parsing `Host(...)` rules).
3. **Publishes via Avahi**: For each discovered domain, publishes an `_http._tcp` or `_https._tcp` mDNS service record via Avahi's D-Bus interface. This makes the domain resolvable as `<domain>.local` on the LAN.
4. **Unpublishes stale entries**: When a container stops or its labels change, removes the corresponding mDNS records.
5. **Cleans up**: Removes stale Avahi publications from previous runs on startup.

## Commands

```
aramdns run                     # Start daemon
```

## Configuration

aramdns uses CLI flags (no config file):

| Flag | Description |
|------|-------------|
| `--verbose` | Debug logging |
| `--runtime <runtime>` | Container runtime: `docker` or `podman` (auto-detected if omitted) |

## How Domains Flow

```
aradeploy deploys app with routing
  → Traefik labels injected into docker-compose.yml
    → aramdns discovers labels on running containers
      → Avahi publishes domain.local via mDNS
        → LAN clients resolve domain.local
```

Example: If aradeploy deploys an app with `routing.subdomain: nextcloud` and `routing.domain: home.local`, aramdns publishes `nextcloud.home.local` via mDNS.

## Interactions with Other Tools

- **aradeploy**: aramdns reads Traefik labels that aradeploy injects into docker-compose.yml files. It does not communicate with aradeploy directly — it watches Docker containers.
- **Docker/Podman**: Queries the container runtime API to list running containers and inspect their labels.
- **Avahi**: Publishes and unpublishes mDNS service records via D-Bus. Manages Avahi configuration to ensure correct network interface binding.
- **aramanager**: Manages aramdns's systemd service. aramdns depends on `docker.service` in its systemd unit.
