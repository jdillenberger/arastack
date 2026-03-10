# Getting Started

This guide takes you from a fresh Linux server to a running app in about 5 minutes.

## Prerequisites

- A Linux server (Debian/Ubuntu recommended, also works on Arch, Fedora, etc.)
- Docker and Docker Compose installed
- `curl` and `git` available
- Root or sudo access

### Install Docker (if needed)

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Log out and back in for the group change to take effect
```

## Step 1: Install arastack

Download and install `aramanager`, then run the full setup automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/jdillenberger/arastack/main/install.sh | sudo bash
```

This will:
1. Create the `arastack` system group and add your user to it
2. Create required directories (`/opt/aradeploy/`, `/etc/arastack/config/`, etc.)
3. Download all arastack binaries
4. Run health checks and auto-fix missing dependencies
5. Install and start systemd services for each tool

When it's done, all services are running. You can verify with:

```bash
aramanager service status
```

## Step 2: Deploy Your First App

### Deploy Nextcloud

```bash
aradeploy deploy nextcloud
```

The interactive wizard will ask you for configuration values (admin password, ports, etc.). Sensible defaults are provided and secrets like passwords are auto-generated.

To skip the wizard and accept all defaults:

```bash
aradeploy deploy nextcloud --quick --yes
```

Once deployed, Nextcloud is available at the port shown in the deployment output. If you have routing enabled (the default), it's also available at `nextcloud.local` on your LAN.

### Deploy WordPress

```bash
aradeploy deploy wordpress
```

Same flow — wizard collects your preferences, renders the template, starts the containers.

### What Just Happened?

When you ran `aradeploy deploy`, it:

1. **Loaded the template** from the [arastack-templates](https://github.com/jdillenberger/arastack-templates) repository
2. **Collected your values** (or used defaults/auto-generated secrets)
3. **Rendered a `docker-compose.yml`** with your values filled in
4. **Injected Traefik labels** for reverse proxy routing (if routing is enabled)
5. **Created the Docker network** if it didn't exist
6. **Started the containers** with `docker compose up -d`
7. **Saved deployment state** to `/opt/aradeploy/apps/<app>/.aradeploy.yaml`

Now the other arastack services automatically pick up your app:
- **arabackup** discovers it and includes it in nightly backups (if backup labels are set)
- **araalert** monitors its container health every 5 minutes
- **aramdns** publishes its domain via mDNS so it's reachable as `<app>.local`
- **aradashboard** shows it in the web UI

## Step 3: Check the Dashboard

Open your browser and go to:

```
http://<your-server-ip>:8420
```

Or if mDNS is working on your network:

```
http://<hostname>.local:8420
```

The dashboard shows all deployed apps, their health status, backup status, and alert history.

## Step 4: Set Up Notifications (Optional)

Configure at least one notification channel so you get alerted when something goes wrong. Edit the config:

```bash
sudo nano /etc/arastack/config/aranotify.yaml
```

Example with ntfy (easiest — free push notifications to your phone):

```yaml
server:
  port: 7140
  bind: 127.0.0.1

channels:
  ntfy:
    url: https://ntfy.sh/my-homelab-alerts
```

Then restart the service:

```bash
sudo aramanager service restart aranotify
```

Test it:

```bash
aranotify send
```

See [aranotify docs](aranotify.md) for email, webhook, and Mattermost setup.

## Step 5: Configure Backups (Optional)

By default, arabackup runs nightly at 3am and backs up all apps that have backup labels in their docker-compose templates. The default backup location is `/mnt/backup/borg/`.

To change the backup schedule or location:

```bash
sudo nano /etc/arastack/config/arabackup.yaml
```

See [arabackup docs](arabackup.md) for the full configuration reference.

## Common Commands

### App Management

```bash
aradeploy list                  # List deployed apps
aradeploy status                # Show container status
aradeploy logs <app> -f         # Follow app logs
aradeploy stop <app>            # Stop an app
aradeploy start <app>           # Start an app
aradeploy restart <app>         # Restart an app
aradeploy remove <app>          # Remove an app (keeps data)
aradeploy remove <app> --purge-data  # Remove app and its data
```

### Templates

```bash
aradeploy templates list        # See all available app templates
aradeploy info <app>            # Show template details before deploying
```

### Backups

```bash
arabackup status                # Show backup status for all apps
arabackup backup                # Run backup now (all apps)
arabackup backup <app>          # Backup a specific app
arabackup list <app>            # List available backup archives
arabackup restore <app>         # Restore from latest backup
```

### System Health

```bash
aramanager doctor               # Run health checks on all tools
aramanager service status       # Show status of all services
aramanager update --check       # Check for arastack updates
aramanager update               # Update all arastack tools
```

## Next Steps

- **[Deploying Apps](deploying-apps.md)** — Deep dive into how deployments work, templates, data storage, routing, and customization
- **[Without arastack](without-arastack.md)** — How to achieve the same with plain Docker Compose and Traefik
- **[aradeploy reference](aradeploy.md)** — Full CLI and configuration reference
- **[arabackup reference](arabackup.md)** — Backup configuration, scheduling, and retention
- **[aranotify reference](aranotify.md)** — Set up email, webhooks, and other notification channels
