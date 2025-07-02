# Momenarr

A lightweight alternative to Sonarr/Radarr with minimal resource consumption. Perfect for running on resource-constrained devices like OpenWRT routers.

## Table of Contents

- [Features](#features)
- [How It Works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
  - [Container Usage](#container-usage)
  - [Building from Source](#building-from-source)
- [Configuration](#configuration)
- [API Endpoints](#api-endpoints)
- [License](#license)

## Features

- **Lightweight**: Minimal resource usage compared to traditional *arr applications
- **Trakt Integration**: Uses Trakt watchlists, favorites, and watch history
- **Automated Downloads**: Searches NZB indexers and sends to NZBGet
- **Smart Quality Selection**: Prioritizes REMUX files, falls back to WEB-DL
- **Automatic Cleanup**: Removes watched media after 5 days
- **TV Show Management**: Downloads next episodes based on watchlist/favorites status

## How It Works

Momenarr integrates with Trakt to manage your media library automatically:

1. **Monitoring**: Periodically checks Trakt watchlists, favorites, and watch history
2. **Searching**: Searches your Newznab indexer for requested content
3. **Downloading**: Sends NZB files to NZBGet for download
4. **Processing**: Copies completed downloads to your media directory
5. **Cleanup**: Removes watched media from disk after 5 days

### TV Shows
- **Watchlist**: Downloads the first unwatched episode
- **Favorites**: Downloads the next 3 episodes

### Movies
- Downloads regardless of watchlist or favorites status

### Quality Selection
1. Prioritizes largest REMUX files first
2. Falls back to smaller REMUX files if download fails
3. Uses WEB-DL versions if no REMUX available

![Workflow Diagram](momenarr.svg)

## Prerequisites

To use Momenarr, you'll need:

1. **Newznab Indexer**: For searching NZB files
2. **NZBGet**: For downloading NZB files
3. **Trakt Account**: With API application created
4. **Media Player**: That supports Trakt (like Infuse)
5. **Media Server** (optional): Such as [WebDAV](https://github.com/amaumene/my_webdav)

## Installation

### Container Usage

> **Note**: You can replace `podman` with `docker` in the command below if you prefer to use Docker.

```bash
podman run --restart unless-stopped -d --name momenarr \
  -v $WHERE_TO_STORE_MEDIAS:/data \
  -v $WHERE_TO_STORE_DB_AND_TRAKT_TOKEN:/config \
  -e DOWNLOAD_DIR="/data/momenarr" \
  -e DATA_DIR="/config" \
  -e NEWSNAB_API_KEY="$YOUR_NEWSNAB_API_KEY" \
  -e NEWSNAB_HOST="$YOUR_NEWSNAB_HOST" \
  -e NZBGET_HOST="$YOUR_NZBGET_HOST" \
  -e NZBGET_PORT="$YOUR_NZBGET_PORT" \
  -e NZBGET_USERNAME="$YOUR_NZBGET_USERNAME" \
  -e NZBGET_PASSWORD="$YOUR_NZBGET_PASSWORD" \
  -e TRAKT_API_KEY="$YOUR_TRAKT_API_KEY" \
  -e TRAKT_CLIENT_SECRET="$YOUR_TRAKT_CLIENT_SECRET" \
  ghcr.io/amaumene/momenarr:main
```

### Building from Source

```bash
git clone https://github.com/amaumene/momenarr.git
cd momenarr
go build -o momenarr
```

## Configuration

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `DOWNLOAD_DIR` | Directory to store downloaded media | Yes |
| `DATA_DIR` | Directory for database and tokens | Yes |
| `NEWSNAB_API_KEY` | Your Newznab indexer API key | Yes |
| `NEWSNAB_HOST` | Your Newznab indexer hostname | Yes |
| `NZBGET_HOST` | NZBGet server hostname | Yes |
| `NZBGET_PORT` | NZBGet server port | Yes |
| `NZBGET_USERNAME` | NZBGet username | Yes |
| `NZBGET_PASSWORD` | NZBGet password | Yes |
| `TRAKT_API_KEY` | Your Trakt API key | Yes |
| `TRAKT_CLIENT_SECRET` | Your Trakt client secret | Yes |

### Trakt Setup

1. Create a Trakt application at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications)
2. Note your Client ID and Client Secret
3. Configure your media player to sync with Trakt

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/notify` | POST | NZBGet notification endpoint for completed downloads |
| `/refresh` | GET | Manually trigger a full refresh of watchlists and cleanup |

## License

This project is licensed under the GPLv3 License - see the [LICENSE](LICENSE) file for details.
