# Momenarr Database Viewer

A standalone tool to view and analyze the Momenarr media database.

## Features

- 📊 **Statistics**: View collection statistics and completion percentage
- 🎬 **Media Listing**: Browse movies and TV episodes with detailed information
- 💾 **NZB Information**: View available NZB files for each media item
- 🎨 **Colored Output**: Beautiful terminal output with status indicators
- 🔍 **Filtering**: Filter by media type, availability status
- 📈 **Sorting**: Sort by title, year, creation date, or status

## Usage

### Build the viewer
```bash
go build -o dbviewer ./cmd/dbviewer
```

### Basic usage
```bash
# View all media in the collection
./dbviewer -db /path/to/data.db

# Show only statistics
./dbviewer -db /path/to/data.db -stats

# Show only movies that are available on disk
./dbviewer -db /path/to/data.db -movies -ondisk

# Show TV shows with NZB information
./dbviewer -db /path/to/data.db -shows -nzbs

# Sort by year (newest first)
./dbviewer -db /path/to/data.db -sort year

# Disable colors (for piping to files)
./dbviewer -db /path/to/data.db -no-color > collection.txt
```

## Command Line Options

| Flag | Description |
|------|-------------|
| `-db` | Path to the database file (required) |
| `-stats` | Show only collection statistics |
| `-movies` | Show only movies |
| `-shows` | Show only TV shows/episodes |
| `-nzbs` | Include NZB file information |
| `-ondisk` | Show only media available on disk |
| `-sort` | Sort by: `title`, `year`, `created`, `updated`, `status` |
| `-no-color` | Disable colored output |

## Output Format

The viewer displays:
- **Media Type**: 🎬 for movies, 📺 for TV episodes
- **Status**: ✅ Available, ⏳ Wanted, ⬇️ Downloading
- **Details**: Year, season/episode, IMDB ID, download status
- **File Information**: File path for available media
- **Timestamps**: Creation and last update times
- **NZB Files**: Available download sources (when `-nzbs` flag is used)

## Examples

### View collection statistics
```bash
$ ./dbviewer -db data.db -stats

╔══════════════════════════════════════════════════════════════════════════════╗
║                          MOMENARR DATABASE VIEWER                          ║
╚══════════════════════════════════════════════════════════════════════════════╝
Database: data.db
Showing: 156 of 156 items
Scanned: 2024-01-15 14:30:45

📊 COLLECTION STATISTICS
  Total Items:     156
  Movies:          89
  TV Episodes:     67
  Unique Shows:    12
  Available:       134
  Wanted:          18
  Downloading:     4
  Completion:      85.9%
```

### View specific media types
```bash
# Show only movies on disk, sorted by year
$ ./dbviewer -db data.db -movies -ondisk -sort year

# Show only wanted TV episodes
$ ./dbviewer -db data.db -shows | grep WANTED
```

## Integration

The database viewer can be used in scripts for automation:

```bash
#!/bin/bash
# Check collection completion
completion=$(./dbviewer -db data.db -stats -no-color | grep "Completion:" | awk '{print $2}' | tr -d '%')
if (( $(echo "$completion < 80" | bc -l) )); then
    echo "Collection completion is below 80%: $completion%"
fi
```