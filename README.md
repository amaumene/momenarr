# Momenarr

Media automation tool that syncs with Trakt watchlists and downloads via Premiumize.

## What it does

Monitors your Trakt lists, finds NZBs from indexers, sends them to Premiumize for cloud downloading. Cleans up watched stuff automatically.

## Setup

Set these environment variables:
```
DOWNLOAD_DIR=/path/to/downloads
DATA_DIR=/path/to/data
NEWSNAB_HOST=your-indexer.com
NEWSNAB_API_KEY=your-api-key
PREMIUMIZE_API_KEY=your-premiumize-key
TRAKT_API_KEY=your-trakt-key
TRAKT_CLIENT_SECRET=your-trakt-secret
```

Run it:
```bash
go build -o momenarr ./cmd/momenarr
./momenarr
```

Or use Docker:
```bash
docker run -d --name momenarr -p 8080:8080 \
  -v /data:/data -v /config:/config \
  -e DOWNLOAD_DIR=/data -e DATA_DIR=/config \
  # ... other env vars
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

Every 6 hours (configurable) it checks Trakt, searches for NZBs, picks best quality (prefers remux > web-dl > biggest file), sends to Premiumize. Files stay in cloud for streaming. Removes stuff you watched in last 5 days.

## License

GPL-3.0