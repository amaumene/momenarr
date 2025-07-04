# Momenarr

A lightweight, efficient media automation tool that integrates with Trakt for automated downloading and management of movies and TV shows. Designed to be resource-efficient and perfect for running on constrained devices like OpenWRT routers.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
  - [Docker/Podman](#dockerpodman)
  - [Building from Source](#building-from-source)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Workflow](#workflow)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Features

- **🚀 Lightweight**: Minimal resource usage with embedded database (BoltDB)
- **📺 Trakt Integration**: Syncs with watchlists, favorites, and watch history
- **🔍 Smart Search**: Searches Newsnab-compatible indexers for NZB files
- **📥 Automated Downloads**: Integrates with NZBGet for download management
- **🎯 Quality Prioritization**: Prefers REMUX > WEB-DL > size-based selection
- **🧹 Auto-Cleanup**: Removes watched media after configurable period
- **🔄 Continuous Sync**: Runs on configurable intervals (default: 6 hours)
- **🌐 REST API**: Full control via HTTP endpoints
- **🛡️ Graceful Shutdown**: Proper cleanup and context cancellation

## Architecture

Momenarr follows a clean, modular architecture:

```
momenarr/
├── cmd/momenarr/        # Application entry point
├── pkg/
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP request handlers
│   ├── models/          # Data models
│   ├── repository/      # Data access layer
│   └── services/        # Business logic
├── internal/            # Internal packages
│   ├── jsonrpc/        # JSON-RPC client
│   └── querystring/    # URL query encoding
├── trakt/              # Trakt API client
├── nzbget/             # NZBGet API client
└── newsnab/            # Newsnab API client
```

## Installation

### Docker/Podman

```bash
# Using Docker
docker run -d \
  --name momenarr \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /path/to/media:/data \
  -v /path/to/config:/config \
  -e DOWNLOAD_DIR="/data" \
  -e DATA_DIR="/config" \
  -e NEWSNAB_HOST="your-indexer.com" \
  -e NEWSNAB_API_KEY="your-api-key" \
  -e NZBGET_HOST="nzbget.local" \
  -e NZBGET_PORT="6789" \
  -e NZBGET_USERNAME="nzbget" \
  -e NZBGET_PASSWORD="password" \
  -e TRAKT_API_KEY="your-trakt-key" \
  -e TRAKT_CLIENT_SECRET="your-trakt-secret" \
  ghcr.io/amaumene/momenarr:latest
```

### Building from Source

Requirements:
- Go 1.23 or later

```bash
# Clone the repository
git clone https://github.com/amaumene/momenarr.git
cd momenarr

# Build the binary
go build -o momenarr ./cmd/momenarr

# Run with environment variables
export DOWNLOAD_DIR="/path/to/downloads"
export DATA_DIR="/path/to/data"
# ... set other required env vars ...
./momenarr
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DOWNLOAD_DIR` | Directory for downloaded media | - | ✅ |
| `DATA_DIR` | Directory for database and tokens | - | ✅ |
| `BLACKLIST_FILE` | Path to blacklist file | `{DATA_DIR}/blacklist.txt` | ❌ |
| `NEWSNAB_HOST` | Newsnab indexer URL | - | ✅ |
| `NEWSNAB_API_KEY` | Newsnab API key | - | ✅ |
| `NZBGET_HOST` | NZBGet hostname | - | ✅ |
| `NZBGET_PORT` | NZBGet port | `6789` | ✅ |
| `NZBGET_USERNAME` | NZBGet username | - | ✅ |
| `NZBGET_PASSWORD` | NZBGet password | - | ✅ |
| `TRAKT_API_KEY` | Trakt application API key | - | ✅ |
| `TRAKT_CLIENT_SECRET` | Trakt client secret | - | ✅ |
| `MOMENARR_API_KEY` | API key for authentication | - | ❌ |
| `HTTP_ADDR` | HTTP server address | `:8080` | ❌ |
| `SYNC_INTERVAL` | Sync interval duration | `6h` | ❌ |
| `WATCHED_DAYS` | Days to look back for watched items | `5` | ❌ |

### Trakt Setup

1. Create a Trakt app at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications)
2. Set redirect URI to `http://localhost:8080/api/trakt/callback`
3. Note your Client ID (API key) and Client Secret
4. Visit `http://your-server:8080/api/trakt/auth` to authenticate

### NZBGet Configuration

1. Enable JSON-RPC in NZBGet settings
2. Create a user with appropriate permissions
3. Configure Momenarr webhook in NZBGet:
   - Event: `NZB_DOWNLOADED`
   - Command: `curl -X POST http://momenarr:8080/api/notify`

### Blacklist Configuration

Create a `blacklist.txt` file with one pattern per line:
```
cam
hdcam
telesync
```

## API Reference

### Health Check
```
GET /health
```
Returns server health status.

### Media Operations

#### Get Media Status
```
GET /api/media/status
```
Returns status of all tracked media.

#### Get Media Statistics
```
GET /api/media/stats
```
Returns aggregate statistics.

### Download Management

#### Retry Failed Download
```
POST /api/download/retry
Body: {"trakt_id": 12345}
```

#### Cancel Download
```
POST /api/download/cancel
Body: {"download_id": 67890}
```

#### Get Download Status
```
GET /api/download/status/{download_id}
```

### Trakt Integration

#### Initiate OAuth Flow
```
GET /api/trakt/auth
```

#### OAuth Callback
```
GET /api/trakt/callback?code={code}&state={state}
```

### Manual Operations

#### Trigger Full Refresh
```
POST /api/refresh
```
Manually triggers the sync workflow.

#### NZBGet Webhook
```
POST /api/notify
```
Receives notifications from NZBGet.

## Workflow

The application runs the following workflow periodically:

1. **Sync from Trakt**: Fetches current watchlists and favorites
2. **Populate NZBs**: Searches for NZB files for media not on disk
3. **Download**: Sends best quality NZBs to NZBGet
4. **Process Notifications**: Handles success/failure from NZBGet
5. **Cleanup**: Removes media watched in the last N days

### Quality Selection Algorithm

1. Searches for all available NZBs for a media item
2. Filters out blacklisted releases
3. Prioritizes by quality tier:
   - REMUX: Score 3,000,000,000 + file size
   - WEB-DL: Score 2,000,000,000 + file size  
   - Other: Score 1,000,000,000 + file size
4. Selects highest scoring NZB

### TV Show Logic

- **Watchlist**: Downloads next unwatched episode
- **Favorites**: Downloads up to 3 upcoming episodes

## Development

### Project Structure

- `cmd/momenarr/`: Main application entry point
- `pkg/`: Public packages
  - `config/`: Configuration loading and validation
  - `handlers/`: HTTP endpoint handlers
  - `models/`: Core data structures
  - `repository/`: Database abstraction layer
  - `services/`: Business logic implementation
- `internal/`: Private implementation packages
- `trakt/`, `nzbget/`, `newsnab/`: External API clients

### Database Viewer

A standalone tool is included to view and analyze your media collection:

```bash
# Build the database viewer
go build -o dbviewer ./cmd/dbviewer

# View collection statistics
./dbviewer -db /path/to/data.db -stats

# View all movies on disk
./dbviewer -db /path/to/data.db -movies -ondisk

# View TV shows with NZB information
./dbviewer -db /path/to/data.db -shows -nzbs

# Use the convenience script
./scripts/view-collection.sh /path/to/data.db
```

See [`cmd/dbviewer/README.md`](cmd/dbviewer/README.md) for full documentation.

### Running Tests

```bash
go test ./...
```

### Building for Different Platforms

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o momenarr-linux-amd64 ./cmd/momenarr

# Linux ARM64 (for Raspberry Pi 4, etc)
GOOS=linux GOARCH=arm64 go build -o momenarr-linux-arm64 ./cmd/momenarr

# macOS
GOOS=darwin GOARCH=amd64 go build -o momenarr-darwin-amd64 ./cmd/momenarr
```

## Security

### API Authentication

To secure your Momenarr instance, set the `MOMENARR_API_KEY` environment variable. When set, all API requests (except health checks and OAuth callbacks) will require authentication:

```bash
# Using Authorization header
curl -H "Authorization: Bearer your-api-key" http://localhost:8080/api/media/stats

# Using X-API-Key header
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/media/stats
```

### Best Practices

1. **Use HTTPS**: Deploy behind a reverse proxy (nginx, Caddy) with TLS
2. **Secure Storage**: Database and token files are created with restrictive permissions (0600)
3. **API Key**: Generate a strong, random API key for production deployments
4. **Network Isolation**: Run in a secure network segment if possible
5. **Regular Updates**: Keep Momenarr and its dependencies updated

### Reporting Security Issues

Please report security vulnerabilities to the maintainers privately via GitHub Security Advisories.

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Add tests for new functionality
- Update documentation as needed

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [BoltDB](https://github.com/etcd-io/bbolt) for embedded storage
- Uses [Logrus](https://github.com/sirupsen/logrus) for structured logging
- Integrates with [Trakt](https://trakt.tv), [NZBGet](https://nzbget.net), and Newsnab indexers