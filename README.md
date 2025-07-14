# Momenarr

A lightweight media automation tool that integrates with Trakt and AllDebrid for automated downloading and management of movies and TV shows. Designed to be resource-efficient and perfect for running on various systems including containers and low-resource devices.

## Features

- **🚀 Lightweight**: Minimal resource usage with embedded database (BoltDB)
- **📺 Trakt Integration**: Syncs with watchlists, favorites, and watch history
- **🔍 Smart Torrent Search**: Searches multiple torrent providers (YGG, APIBay)
- **☁️ AllDebrid Integration**: Downloads torrents through AllDebrid's premium service
- **🎯 Quality Prioritization**: Prefers REMUX > High Resolution > File Size
- **🧹 Auto-Cleanup**: Removes watched media after configurable period
- **🔄 Continuous Sync**: Runs on configurable intervals (default: 6 hours)
- **🌐 REST API**: Full control via HTTP endpoints
- **🛡️ API Security**: Optional API key authentication

## Architecture

```
momenarr/
├── cmd/
│   ├── momenarr/        # Main application
│   └── dbviewer/        # Database inspection tool
├── pkg/
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP request handlers
│   ├── models/          # Data models
│   ├── repository/      # Data access layer
│   ├── services/        # Business logic
│   └── utils/           # Common utilities
├── bolthold/            # Embedded BoltDB wrapper
└── trakt/               # Trakt API client
```

## Installation

### Docker/Podman

```bash
docker run -d \
  --name momenarr \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /path/to/data:/data \
  -e DATA_DIR="/data" \
  -e ALLDEBRID_API_KEY="your-alldebrid-key" \
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
export DATA_DIR="/path/to/data"
export ALLDEBRID_API_KEY="your-alldebrid-key"
export TRAKT_API_KEY="your-trakt-key"
export TRAKT_CLIENT_SECRET="your-trakt-secret"
./momenarr
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DATA_DIR` | Directory for database and tokens | - | ✅ |
| `ALLDEBRID_API_KEY` | AllDebrid API key | - | ✅ |
| `TRAKT_API_KEY` | Trakt application API key | - | ✅ |
| `TRAKT_CLIENT_SECRET` | Trakt client secret | - | ✅ |
| `HTTP_ADDR` | HTTP server address | `:8080` | ❌ |
| `BLACKLIST_FILE` | Path to blacklist file | `{DATA_DIR}/blacklist.txt` | ❌ |
| `SYNC_INTERVAL` | Sync interval duration | `6h` | ❌ |
| `WATCHED_DAYS` | Days to look back for watched items | `5` | ❌ |
| `MOMENARR_API_KEY` | API key for authentication | - | ❌ |

### Trakt Setup

1. Create a Trakt app at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications)
2. Set redirect URI to `http://localhost:8080/api/trakt/callback`
3. Note your Client ID (API key) and Client Secret
4. Visit `http://your-server:8080/api/trakt/auth` to authenticate

### AllDebrid Setup

1. Get your API key from [alldebrid.com/apikeys](https://alldebrid.com/apikeys)
2. Set the `ALLDEBRID_API_KEY` environment variable

### Blacklist Configuration

Create a `blacklist.txt` file with one pattern per line to exclude certain releases:
```
cam
hdcam
telesync
ts
```

## API Reference

### Media Operations

#### Get All Media
```
GET /api/media
```
Returns an HTML page with all tracked media.

#### Get Media Statistics
```
GET /api/media/stats
```
Returns JSON with aggregate statistics:
```json
{
  "total": 150,
  "on_disk": 120,
  "not_on_disk": 30,
  "movies": 50,
  "episodes": 100,
  "downloading": 5
}
```

### Download Management

#### List Torrents for Media
```
GET /api/torrents/list?trakt_id={id}
```

#### Retry Failed Download
```
POST /api/download/retry?trakt_id={id}
```

#### Cancel Download
```
POST /api/download/cancel?trakt_id={id}
```

#### Get Download Status
```
GET /api/download/status?trakt_id={id}
```

### Manual Operations

#### Trigger Full Refresh
```
GET /api/refresh
```
Manually triggers sync from Trakt and torrent search.

#### Get Cleanup Statistics
```
GET /api/cleanup/stats
```
Returns statistics about watched media eligible for cleanup.

## Workflow

The application runs the following workflow periodically:

1. **Sync from Trakt**: Fetches current watchlists and favorites
2. **Search Torrents**: Searches for torrents for media not on disk
3. **Check AllDebrid Cache**: Verifies if torrents are cached
4. **Download**: Adds selected torrents to AllDebrid
5. **Monitor**: Tracks download progress
6. **Cleanup**: Removes watched media after configured days

### Quality Selection Algorithm

1. Searches all available torrents for a media item
2. Filters out blacklisted releases
3. Prioritizes by:
   - REMUX quality (highest)
   - Resolution (4K > 1080p > 720p)
   - File size (larger preferred)
4. Checks AllDebrid cache availability
5. Selects best cached torrent

### TV Show Logic

- **Watchlist**: Downloads next unwatched episode
- **Favorites**: Downloads complete seasons when available
- **Season Packs**: Intelligently handles multi-episode torrents

## Database Viewer

A standalone tool is included to inspect your media collection:

```bash
# Build the database viewer
go build -o dbviewer ./cmd/dbviewer

# View collection statistics
./dbviewer -db /path/to/data.db -stats

# View all movies
./dbviewer -db /path/to/data.db -movies

# View TV shows
./dbviewer -db /path/to/data.db -shows

# Filter by on-disk status
./dbviewer -db /path/to/data.db -movies -ondisk
./dbviewer -db /path/to/data.db -shows -not-ondisk
```

## Security

### API Authentication

To secure your Momenarr instance, set the `MOMENARR_API_KEY` environment variable:

```bash
export MOMENARR_API_KEY="your-secure-api-key"
```

Then include the key in your requests:
```bash
# Using Authorization header
curl -H "Authorization: Bearer your-api-key" http://localhost:8080/api/media/stats

# Using X-API-Key header
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/media/stats
```

### Best Practices

1. **Use HTTPS**: Deploy behind a reverse proxy with TLS
2. **Secure Storage**: Database files are created with restrictive permissions
3. **API Key**: Generate a strong, random API key for production
4. **Network**: Limit access to trusted networks

## Development

### Running Tests

```bash
go test ./...
```

### Building for Different Platforms

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o momenarr-linux-amd64 ./cmd/momenarr

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o momenarr-linux-arm64 ./cmd/momenarr

# macOS
GOOS=darwin GOARCH=amd64 go build -o momenarr-darwin-amd64 ./cmd/momenarr

# Windows
GOOS=windows GOARCH=amd64 go build -o momenarr.exe ./cmd/momenarr
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [BoltDB](https://github.com/etcd-io/bbolt) for embedded storage
- Uses [Logrus](https://github.com/sirupsen/logrus) for structured logging
- Integrates with [Trakt](https://trakt.tv) and [AllDebrid](https://alldebrid.com)