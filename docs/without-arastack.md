# Without arastack

This guide shows how to achieve the same deployment results as `aradeploy` using plain Docker Compose and Traefik. This covers only the deployment features — backups, monitoring, alerts, and peer discovery are separate concerns.

## What aradeploy Does for You

Before going manual, it helps to understand what aradeploy automates:

1. Renders a `docker-compose.yml` from a template with your values
2. Creates a shared Docker network
3. Deploys and configures Traefik as a reverse proxy
4. Injects Traefik labels into your compose file
5. Manages HTTPS certificates via Let's Encrypt
6. Tracks deployment state

All of this can be done by hand.

## Step 1: Set Up the Docker Network

All apps and Traefik need to be on the same Docker network to communicate:

```bash
docker network create aradeploy-net
```

## Step 2: Set Up Traefik

Create a directory for Traefik:

```bash
mkdir -p /opt/traefik
```

Create `/opt/traefik/docker-compose.yml`:

```yaml
services:
  traefik:
    image: traefik:v3
    restart: unless-stopped
    command:
      - "--api.dashboard=false"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--providers.docker.network=aradeploy-net"
      - "--entrypoints.web.address=:80"
    ports:
      - "80:80"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - aradeploy-net

networks:
  aradeploy-net:
    external: true
```

### With HTTPS

If you want automatic HTTPS certificates, add the ACME configuration:

```yaml
services:
  traefik:
    image: traefik:v3
    restart: unless-stopped
    command:
      - "--api.dashboard=false"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--providers.docker.network=aradeploy-net"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      # Redirect HTTP → HTTPS
      - "--entrypoints.web.http.redirections.entrypoint.to=websecure"
      - "--entrypoints.web.http.redirections.entrypoint.scheme=https"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./letsencrypt:/letsencrypt
    networks:
      - aradeploy-net

networks:
  aradeploy-net:
    external: true
```

Start Traefik:

```bash
cd /opt/traefik && docker compose up -d
```

## Step 3: Deploy an App

### Nextcloud (Manual)

Create the app directory:

```bash
mkdir -p /opt/apps/nextcloud
```

Create `/opt/apps/nextcloud/docker-compose.yml`:

```yaml
services:
  nextcloud:
    image: nextcloud:latest
    restart: unless-stopped
    volumes:
      - nextcloud_data:/var/www/html
    environment:
      NEXTCLOUD_ADMIN_USER: admin
      NEXTCLOUD_ADMIN_PASSWORD: change-me-to-a-secure-password
      POSTGRES_HOST: db
      POSTGRES_DB: nextcloud
      POSTGRES_USER: nextcloud
      POSTGRES_PASSWORD: db-password-here
      NEXTCLOUD_TRUSTED_DOMAINS: "nextcloud.home.local"
    depends_on:
      - db
    labels:
      traefik.enable: "true"
      traefik.http.routers.nextcloud.rule: "Host(`nextcloud.home.local`)"
      traefik.http.services.nextcloud.loadbalancer.server.port: "80"
      # For HTTPS (if Traefik is configured with ACME):
      # traefik.http.routers.nextcloud.tls.certresolver: "letsencrypt"
      # traefik.http.routers.nextcloud.entrypoints: "websecure"
    networks:
      - aradeploy-net
      - default

  db:
    image: postgres:16-alpine
    restart: unless-stopped
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: nextcloud
      POSTGRES_USER: nextcloud
      POSTGRES_PASSWORD: db-password-here

volumes:
  nextcloud_data:
  db_data:

networks:
  aradeploy-net:
    external: true
```

Start it:

```bash
cd /opt/apps/nextcloud && docker compose up -d
```

Nextcloud is now available at `http://nextcloud.home.local` (assuming your DNS or `/etc/hosts` resolves that domain).

### WordPress (Manual)

Create `/opt/apps/wordpress/docker-compose.yml`:

```yaml
services:
  wordpress:
    image: wordpress:latest
    restart: unless-stopped
    volumes:
      - wp_data:/var/www/html
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: db-password-here
      WORDPRESS_DB_NAME: wordpress
    depends_on:
      - db
    labels:
      traefik.enable: "true"
      traefik.http.routers.wordpress.rule: "Host(`wordpress.home.local`)"
      traefik.http.services.wordpress.loadbalancer.server.port: "80"
    networks:
      - aradeploy-net
      - default

  db:
    image: mariadb:11
    restart: unless-stopped
    volumes:
      - db_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: root-password-here
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: db-password-here

volumes:
  wp_data:
  db_data:

networks:
  aradeploy-net:
    external: true
```

```bash
cd /opt/apps/wordpress && docker compose up -d
```

## Step 4: Local DNS Resolution

Without aramdns, you need to resolve `*.home.local` domains yourself. Options:

### Option A: `/etc/hosts` (Quick and Dirty)

On each client machine, add entries to `/etc/hosts`:

```
192.168.1.100  nextcloud.home.local
192.168.1.100  wordpress.home.local
```

### Option B: Local DNS Server

Run a DNS server like Pi-hole or dnsmasq that resolves `*.home.local` to your server's IP.

### Option C: Avahi mDNS (Manual)

Install Avahi and publish services manually:

```bash
sudo apt install avahi-daemon avahi-utils
avahi-publish -s "Nextcloud" _http._tcp 80 --host=nextcloud.local
```

This is exactly what aramdns automates.

## Comparison

| Aspect | With arastack | Without arastack |
|--------|---------------|------------------|
| **Deploy an app** | `aradeploy deploy nextcloud` | Write docker-compose.yml manually |
| **Traefik setup** | Automatic | Manual compose file + configuration |
| **Traefik labels** | Auto-injected from template | Write labels by hand |
| **HTTPS** | Config flag + ACME email | Manual Traefik ACME configuration |
| **Local DNS** | Automatic via aramdns | `/etc/hosts` or separate DNS server |
| **Secrets** | Auto-generated | Generate and manage yourself |
| **Upgrades** | `aradeploy upgrade` | `docker compose pull && docker compose up -d` |
| **Multiple apps** | Shared network auto-created | Create network manually, add to each compose |

## Managing Apps Without arastack

### Start/Stop

```bash
cd /opt/apps/nextcloud
docker compose up -d          # start
docker compose stop           # stop
docker compose down           # stop and remove containers
docker compose down -v        # stop, remove containers and volumes (data loss!)
```

### View Logs

```bash
cd /opt/apps/nextcloud
docker compose logs -f
```

### Update Images

```bash
cd /opt/apps/nextcloud
docker compose pull           # pull latest images
docker compose up -d          # recreate with new images
```

### Backup (Manual)

Without arabackup, you handle backups yourself. A basic approach:

```bash
# Stop the app
cd /opt/apps/nextcloud && docker compose stop

# Backup the data volume
docker run --rm -v nextcloud_nextcloud_data:/data -v /backup:/backup \
  alpine tar czf /backup/nextcloud-$(date +%Y%m%d).tar.gz -C /data .

# Backup the database
docker compose exec db pg_dump -U nextcloud nextcloud > /backup/nextcloud-db-$(date +%Y%m%d).sql

# Start the app again
docker compose start
```

For production use, consider [BorgBackup](https://borgbackup.readthedocs.io/) for deduplication and encryption — which is what arabackup uses under the hood.

## Key Takeaway

arastack wraps these manual steps into a repeatable, automated workflow. The underlying technology is standard Docker Compose + Traefik — there's no lock-in. You can always `aradeploy eject <app>` to get a standalone compose file and manage it yourself.
