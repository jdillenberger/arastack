# aranotify

Multi-channel notification delivery service. aranotify receives notification requests via its API and dispatches them to configured channels.

## How It Works

aranotify runs as a daemon with an HTTP API. When a notification is received (typically from araalert), the dispatcher routes it to all enabled channels. Each channel implementation handles formatting and delivery for its specific protocol.

Notifications include a title, body, severity level, source identifier, and optionally target specific channels.

## Commands

```
aranotify run                   # Start daemon (API server)
aranotify send                  # Send a test notification
aranotify channels              # List configured channels
```

## Configuration

File: `/etc/arastack/config/aranotify.yaml`

```yaml
server:
  port: 7140
  bind: 127.0.0.1

channels:
  webhook:
    url: https://webhook.example.com

  ntfy:
    url: https://ntfy.sh
    token: token123

  email:
    host: smtp.gmail.com
    port: 587
    from: alerts@example.com
    to:
      - admin@example.com
    username: user
    password: pass

  mattermost:
    webhook_url: https://mattermost.example.com/hooks/xyz
```

Environment variable overrides use the `ARANOTIFY_` prefix.

## Notification Channels

| Channel | Protocol | Description |
|---------|----------|-------------|
| **Webhook** | HTTP POST | Sends JSON payload to a URL |
| **Ntfy** | HTTP POST | Push notifications via [ntfy.sh](https://ntfy.sh) |
| **Email** | SMTP | Email delivery with configurable from/to |
| **Mattermost** | HTTP POST | Slack-compatible incoming webhook |

Only channels with configuration present are active. The `channels` command lists which channels are enabled.

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check with version info |
| `/api/send` | POST | Send a notification |

### Notification Payload

```json
{
  "title": "Backup Failed",
  "body": "arabackup failed for app 'nextcloud'",
  "severity": "critical",
  "source": "arabackup",
  "channels": ["webhook", "email"]
}
```

The `channels` field is optional — if omitted, the notification goes to all enabled channels.

## Global Flags

| Flag | Description |
|------|-------------|
| `-v`, `--verbose` | Debug logging |
| `--config <path>` | Config file path |

## Interactions with Other Tools

- **araalert**: Primary consumer. araalert sends notifications when alert rules fire or push events are received.
