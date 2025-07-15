# Momenarr

A lightweight media automation tool that monitors your Trakt watchlist and favorites, automatically searches for torrents, and downloads them through AllDebrid.

## Features

- **Trakt Integration**: Syncs with your watchlist and favorites
- **Automatic Torrent Search**: Searches YGG and APIBay for media
- **AllDebrid Downloads**: Downloads torrents through AllDebrid's premium service
- **Smart Cleanup**: Automatically removes watched content after configurable days
- **Web Interface**: View and manage your media collection
- **Database Viewer**: Command-line tool to inspect your collection
- **API Access**: REST API for external integrations

## Requirements

- Go 1.23+ (for building from source)
- AllDebrid account with API key
- Trakt account with API credentials

## Installation

### Docker

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

```bash
git clone https://github.com/amaumene/momenarr.git
cd momenarr
go build -o momenarr ./cmd/momenarr
```

## Configuration

### Required Environment Variables

| Variable | Description |
|----------|-------------|
| `DATA_DIR` | Directory for database and tokens |
| `ALLDEBRID_API_KEY` | Your AllDebrid API key |
| `TRAKT_API_KEY` | Your Trakt application API key |
| `TRAKT_CLIENT_SECRET` | Your Trakt client secret |

### Optional Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_ADDR` | HTTP server address | `:8080` |
| `BLACKLIST_FILE` | Path to blacklist file | `{DATA_DIR}/blacklist.txt` |
| `SYNC_INTERVAL` | How often to sync | `6h` |
| `WATCHED_DAYS` | Days to look back for watched items | `5` |
| `MOMENARR_API_KEY` | API key for authentication | _(none)_ |

### Setting up Trakt

1. Create a Trakt app at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications)
2. Set redirect URI to `http://localhost:8080/api/trakt/callback`
3. Note your Client ID (use as `TRAKT_API_KEY`) and Client Secret
4. Start momenarr and visit `http://localhost:8080/api/trakt/auth` to authenticate

### Setting up AllDebrid

1. Get your API key from [alldebrid.com/apikeys](https://alldebrid.com/apikeys)
2. Set the `ALLDEBRID_API_KEY` environment variable

### Blacklist

Create a `blacklist.txt` file to exclude certain releases:
```
cam
hdcam
telesync
```

## API Endpoints

All endpoints are prefixed with `/api/`:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/media` | GET | HTML page showing all media |
| `/api/media/stats` | GET | JSON statistics about your collection |
| `/api/torrents/list?trakt_id=X` | GET | List available torrents for a media item |
| `/api/download/retry?trakt_id=X` | POST | Retry a failed download |
| `/api/download/cancel?trakt_id=X` | POST | Cancel an active download |
| `/api/download/status?trakt_id=X` | GET | Get download status |
| `/api/refresh` | GET | Manually trigger sync and search |
| `/api/cleanup/stats` | GET | Statistics about watched media |

### API Authentication

If `MOMENARR_API_KEY` is set, include it in requests:
```bash
curl -H "Authorization: Bearer your-api-key" http://localhost:8080/api/media/stats
# or
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/media/stats
```

## Database Viewer

A command-line tool is included to inspect your media database:

```bash
# Build the viewer
go build -o dbviewer ./cmd/dbviewer

# Show statistics
./dbviewer -db /path/to/data/data.db -stats

# List all movies
./dbviewer -db /path/to/data/data.db -movies

# List TV shows
./dbviewer -db /path/to/data/data.db -shows

# Additional options:
#   -ondisk      Show only media on disk
#   -not-ondisk  Show only missing media
#   -sort        Sort by: title, year, added (default: title)
#   -limit N     Limit results
```

## How It Works

1. **Sync**: Periodically fetches your Trakt watchlist and favorites
2. **Search**: For each missing media item, searches torrents on YGG and APIBay
3. **Download**: Sends the best torrent to AllDebrid (checks cache first)
4. **Monitor**: Tracks download progress
5. **Cleanup**: Removes media that has been watched (after configured days)

The application maintains a BoltDB database tracking:
- Media items (movies and TV episodes)
- Torrent information
- Download status
- Watch history

## Development

### Project Structure

```
momenarr/
├── cmd/
│   ├── momenarr/      # Main application
│   └── dbviewer/      # Database viewer tool
├── pkg/
│   ├── alldebrid/     # AllDebrid API client
│   ├── config/        # Configuration
│   ├── handlers/      # HTTP handlers
│   ├── models/        # Data models
│   ├── repository/    # Database layer
│   ├── services/      # Business logic
│   └── utils/         # Utilities
├── bolthold/          # BoltDB wrapper
└── trakt/             # Trakt API client
```

### Building

```bash
# Run tests
go test ./...

# Build for current platform
go build -o momenarr ./cmd/momenarr

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o momenarr-linux-amd64 ./cmd/momenarr
GOOS=darwin GOARCH=arm64 go build -o momenarr-darwin-arm64 ./cmd/momenarr
```

## License

GNU General Public License v3.0 - see [LICENSE](LICENSE) file for details.