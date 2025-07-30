# Momenarr

A lightweight media automation tool that monitors your Trakt watchlist and favorites, automatically searches for torrents, and downloads them through AllDebrid.

## Features

- **Trakt Integration**: Syncs with your watchlist and favorites
- **Multi-Provider Search**: Searches APIBay (international) and YGG (French content)
- **Language-Aware**: Automatically selects the best torrent provider based on content language
- **TMDB Integration**: Enhanced metadata with French translations and original language detection
- **AllDebrid Downloads**: Downloads torrents through AllDebrid's premium service
- **Smart Cleanup**: Automatically removes watched content after configurable days
- **Web Interface**: View and manage your media collection
- **Database Viewer**: Command-line tool to inspect your collection
- **REST API**: Full API for external integrations

## Requirements

- Go 1.24+ (for building from source)
- AllDebrid account with API key
- Trakt account with API credentials
- TMDB API key (optional, for enhanced metadata)

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
  -e TMDB_API_KEY="your-tmdb-key" \
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
| `TMDB_API_KEY` | TMDB API for metadata and translations | _(none)_ |
| `BLACKLIST_FILE` | Path to blacklist file | `{DATA_DIR}/blacklist.txt` |
| `SYNC_INTERVAL` | How often to sync with Trakt | `6h` |
| `WATCHED_DAYS` | Days to keep watched items before cleanup | `5` |
| `MAX_RETRIES` | Maximum download retry attempts | `3` |
| `REQUEST_TIMEOUT` | HTTP request timeout in seconds | `30` |

### Setting up Trakt

1. Create a Trakt app at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications)
2. Set redirect URI to `http://localhost:8080/api/trakt/callback`
3. Note your Client ID (use as `TRAKT_API_KEY`) and Client Secret
4. Start momenarr and visit `http://localhost:8080/api/trakt/auth` to authenticate

### Setting up AllDebrid

1. Get your API key from [alldebrid.com/apikeys](https://alldebrid.com/apikeys)
2. Set the `ALLDEBRID_API_KEY` environment variable

### Setting up TMDB (Optional but Recommended)

1. Get your API key from [themoviedb.org/settings/api](https://www.themoviedb.org/settings/api)
2. Set the `TMDB_API_KEY` environment variable
3. This enables:
   - Original language detection for better provider selection
   - French title translations for French content
   - Enhanced metadata

### Blacklist

Create a `blacklist.txt` file to exclude certain releases:
```
cam
hdcam
telesync
ts
screener
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
| `/api/trakt/auth` | GET | Initiate Trakt authentication |
| `/api/trakt/callback` | GET | Trakt OAuth callback |

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
2. **Metadata Enhancement**: If TMDB is configured, fetches original language and French titles
3. **Smart Search**: 
   - For French content (detected via TMDB): Uses YGG (French tracker)
   - For international content: Uses APIBay
   - Automatic fallback if preferred provider unavailable
4. **Download**: Sends the best torrent to AllDebrid (prioritizes cached torrents)
5. **Monitor**: Tracks download progress and handles failures
6. **Cleanup**: Removes media that has been watched (after configured days)

The application maintains a BoltDB database tracking:
- Media items with enhanced metadata (TMDB ID, original language, French titles)
- Torrent information from multiple providers
- Download status and history
- Watch history from Trakt

## Provider Details

### APIBay
- International torrent provider
- Used for non-French content
- Searches The Pirate Bay database

### YGG (YggTorrent)
- French torrent tracker
- Used for French content when original language is detected
- Requires no authentication
- Supports hash caching for better performance

## Development

### Project Structure

```
momenarr/
├── cmd/
│   ├── momenarr/      # Main application
│   └── dbviewer/      # Database viewer tool
├── pkg/
│   ├── config/        # Configuration management
│   ├── handlers/      # HTTP handlers
│   ├── models/        # Data models
│   ├── repository/    # Database layer
│   ├── services/      # Business logic
│   │   ├── alldebrid_client.go     # AllDebrid API client
│   │   ├── alldebrid_service.go    # AllDebrid service layer
│   │   ├── app_service.go          # Main application service
│   │   ├── cleanup_service.go      # Cleanup logic
│   │   ├── download_service.go     # Download management
│   │   ├── tmdb_service.go         # TMDB integration
│   │   ├── torrent_search_service.go # Search orchestration
│   │   ├── torrent_service.go      # Torrent management
│   │   ├── trakt_service.go        # Trakt integration
│   │   ├── apibay_provider.go      # APIBay search provider
│   │   └── ygg_provider.go         # YGG search provider
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

# Build with version info
go build -ldflags "-X main.Version=1.0.0" -o momenarr ./cmd/momenarr

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o momenarr-linux-amd64 ./cmd/momenarr
GOOS=darwin GOARCH=arm64 go build -o momenarr-darwin-arm64 ./cmd/momenarr
GOOS=windows GOARCH=amd64 go build -o momenarr.exe ./cmd/momenarr
```

### Code Quality

The codebase follows Go best practices:
- Clean architecture with separated concerns
- Interface-based design for testability
- Comprehensive error handling
- Structured logging with logrus
- Concurrent operations where appropriate

## License

GNU General Public License v3.0 - see [LICENSE](LICENSE) file for details.