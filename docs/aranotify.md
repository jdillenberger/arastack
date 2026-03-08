# aranotify

Notification delivery service. Receives notification requests via REST API and dispatches them to configured channels (webhook, email, ntfy, Mattermost).

## Commands

| Command | Description |
|---------|-------------|
| `aranotify run` | Run the notification daemon |
| `aranotify send` | Send a test notification |
| `aranotify channels` | List configured notification channels |

## Configuration

Default config path: `/etc/arastack/config/aranotify.yaml`

### Server

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `7140` | API server port |
| `server.bind` | `127.0.0.1` | Bind address |

### Notification Channels

#### Webhook
| Key | Description |
|-----|-------------|
| `channels.webhook.url` | Webhook URL for generic HTTP POST notifications |

#### ntfy
| Key | Description |
|-----|-------------|
| `channels.ntfy.url` | ntfy.sh service URL |
| `channels.ntfy.token` | ntfy authentication token |

#### Email
| Key | Default | Description |
|-----|---------|-------------|
| `channels.email.host` | - | SMTP host |
| `channels.email.port` | `587` | SMTP port |
| `channels.email.from` | - | Sender address |
| `channels.email.to` | - | Recipient address list |
| `channels.email.username` | - | SMTP username |
| `channels.email.password` | - | SMTP password |

#### Mattermost
| Key | Description |
|-----|-------------|
| `channels.mattermost.webhook_url` | Mattermost incoming webhook URL |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check |
| `POST` | `/api/send` | Send notification to a channel |
| `GET` | `/api/channels` | List available channels |
| `POST` | `/api/test` | Send test notification |

## Interactions with Other Tools

- **araalert** - the primary consumer. araalert dispatches alert notifications to aranotify's `/api/send` endpoint when alert rules trigger.
