# Momenarr

Media automation tool that syncs with Trakt watchlists and manages cloud storage via Premiumize.

## What it does

Monitors your Trakt lists, finds NZBs from Usenet indexers, sends them to Premiumize for cloud storage. Files remain in Premiumize cloud for streaming - no local downloads. Automatically cleans up watched content.

## Setup

Set these environment variables:
```
# Required
DATA_DIR=/path/to/data              # Database and config storage
NEWSNAB_HOST=your-indexer.com       # Usenet indexer URL
NEWSNAB_API_KEY=your-api-key        # Usenet indexer API key
PREMIUMIZE_API_KEY=your-key         # Premiumize API key
TRAKT_API_KEY=your-trakt-key        # Trakt API key
TRAKT_CLIENT_SECRET=your-secret     # Trakt client secret

# Optional (with defaults)
PORT=3000                            # Web server port
HOST=0.0.0.0                        # Web server host
SYNC_INTERVAL=6h                    # How often to sync
BLACKLIST_FILE=blacklist.txt        # Path to blacklist file
MAX_RETRIES=3                        # Max retry attempts
REQUEST_TIMEOUT=30                   # Request timeout in seconds
```

Run it:
```bash
go build -o momenarr ./cmd/momenarr
./momenarr
```

Or use Docker:
```bash
docker run -d --name momenarr -p 8080:8080 \
  -v /config:/config \
  -e DATA_DIR=/config \
  -e NEWSNAB_HOST=your-indexer.com \
  -e NEWSNAB_API_KEY=your-api-key \
  -e PREMIUMIZE_API_KEY=your-key \
  -e TRAKT_API_KEY=your-trakt-key \
  -e TRAKT_CLIENT_SECRET=your-secret \
  ghcr.io/amaumene/momenarr:latest
```

## Trakt Auth

1. Create app at trakt.tv/oauth/applications
2. Set redirect to http://localhost:8080/api/trakt/callback
3. Go to http://your-server:8080/api/trakt/auth to connect

## API

- `GET /health` - check if running
- `GET /api/media/status` - see what's tracked
- `POST /api/refresh` - force sync now
- `POST /api/download/retry` - retry failed download

## How it works

Every 6 hours (configurable via SYNC_INTERVAL) it:
1. Syncs with Trakt watchlist and favorites
2. Searches Usenet indexers for NZBs
3. Picks best quality (prefers remux > web-dl > biggest file)
4. Sends NZB to Premiumize for cloud storage
5. Monitors transfer status
6. Marks media as available when ready in Premiumize cloud
7. Auto-removes watched content from Premiumize after 5 days

**Note:** Files remain in Premiumize cloud for streaming. No local downloads are performed.

## License

GPL-3.0