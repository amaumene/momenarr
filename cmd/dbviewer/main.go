package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/models"
)

// Colors for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// MediaStats holds statistics about the media collection
type MediaStats struct {
	TotalItems   int
	Movies       int
	Episodes     int
	OnDisk       int
	NotOnDisk    int
	Downloading  int
	UniqueShows  int
}

func main() {
	var (
		dbPath     = flag.String("db", "", "Path to the database file (required)")
		showStats  = flag.Bool("stats", false, "Show only statistics")
		showMovies = flag.Bool("movies", false, "Show only movies")
		showShows  = flag.Bool("shows", false, "Show only TV shows")
		showNZBs   = flag.Bool("nzbs", false, "Show NZB information")
		onDiskOnly = flag.Bool("ondisk", false, "Show only media on disk")
		noColor    = flag.Bool("no-color", false, "Disable colored output")
		sortBy     = flag.String("sort", "title", "Sort by: title, year, created, updated, status")
	)
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -db <database-path> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -db /path/to/data.db -stats\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -db /path/to/data.db -movies -ondisk\n", os.Args[0])
		os.Exit(1)
	}

	// Check if database file exists
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Database file '%s' does not exist\n", *dbPath)
		os.Exit(1)
	}

	// Open database
	store, err := bolthold.Open(*dbPath, 0600, &bolthold.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Fetch all media
	var mediaItems []*models.Media
	err = store.Find(&mediaItems, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading media from database: %v\n", err)
		os.Exit(1)
	}

	// Filter media based on flags
	filteredMedia := filterMedia(mediaItems, *showMovies, *showShows, *onDiskOnly)
	
	// Sort media
	sortMedia(filteredMedia, *sortBy)

	// Calculate statistics
	stats := calculateStats(mediaItems)

	// Set color functions
	colorize := getColorizer(*noColor)

	// Print header
	printHeader(colorize, *dbPath, len(filteredMedia), len(mediaItems))

	if *showStats {
		printStatistics(colorize, stats)
		return
	}

	// Print media collection
	printMediaCollection(colorize, filteredMedia, *showNZBs, store)

	// Print summary statistics
	fmt.Printf("\n" + colorize("cyan", "=== SUMMARY ===") + "\n")
	printStatistics(colorize, stats)
}

func filterMedia(media []*models.Media, moviesOnly, showsOnly, onDiskOnly bool) []*models.Media {
	var filtered []*models.Media
	
	for _, item := range media {
		// Filter by type
		if moviesOnly && !item.IsMovie() {
			continue
		}
		if showsOnly && !item.IsEpisode() {
			continue
		}
		
		// Filter by on-disk status
		if onDiskOnly && !item.OnDisk {
			continue
		}
		
		filtered = append(filtered, item)
	}
	
	return filtered
}

func sortMedia(media []*models.Media, sortBy string) {
	sort.Slice(media, func(i, j int) bool {
		switch sortBy {
		case "year":
			if media[i].Year != media[j].Year {
				return media[i].Year > media[j].Year
			}
			return media[i].Title < media[j].Title
		case "created":
			return media[i].CreatedAt.After(media[j].CreatedAt)
		case "updated":
			return media[i].UpdatedAt.After(media[j].UpdatedAt)
		case "status":
			statusI := getStatusPriority(media[i])
			statusJ := getStatusPriority(media[j])
			if statusI != statusJ {
				return statusI < statusJ
			}
			return media[i].Title < media[j].Title
		default: // title
			return media[i].Title < media[j].Title
		}
	})
}

func getStatusPriority(media *models.Media) int {
	if media.OnDisk {
		return 1 // Available
	}
	if media.DownloadID > 0 {
		return 2 // Downloading
	}
	return 3 // Wanted
}

func calculateStats(media []*models.Media) MediaStats {
	stats := MediaStats{}
	shows := make(map[string]bool)
	
	for _, item := range media {
		stats.TotalItems++
		
		if item.IsMovie() {
			stats.Movies++
		} else {
			stats.Episodes++
			// Extract show title (remove episode info)
			showTitle := item.Title
			if strings.Contains(showTitle, " - ") {
				parts := strings.Split(showTitle, " - ")
				if len(parts) > 0 {
					showTitle = parts[0]
				}
			}
			shows[showTitle] = true
		}
		
		if item.OnDisk {
			stats.OnDisk++
		} else {
			stats.NotOnDisk++
		}
		
		if item.DownloadID > 0 {
			stats.Downloading++
		}
	}
	
	stats.UniqueShows = len(shows)
	return stats
}

func getColorizer(noColor bool) func(string, string) string {
	if noColor {
		return func(color, text string) string { return text }
	}
	
	colors := map[string]string{
		"red":    ColorRed,
		"green":  ColorGreen,
		"yellow": ColorYellow,
		"blue":   ColorBlue,
		"purple": ColorPurple,
		"cyan":   ColorCyan,
		"white":  ColorWhite,
		"bold":   ColorBold,
	}
	
	return func(color, text string) string {
		if c, ok := colors[color]; ok {
			return c + text + ColorReset
		}
		return text
	}
}

func printHeader(colorize func(string, string) string, dbPath string, filtered, total int) {
	fmt.Printf(colorize("bold", "╔══════════════════════════════════════════════════════════════════════════════╗\n"))
	fmt.Printf(colorize("bold", "║") + colorize("cyan", "                          MOMENARR DATABASE VIEWER                          ") + colorize("bold", "║\n"))
	fmt.Printf(colorize("bold", "╚══════════════════════════════════════════════════════════════════════════════╝\n"))
	fmt.Printf(colorize("yellow", "Database: ") + "%s\n", filepath.Base(dbPath))
	fmt.Printf(colorize("yellow", "Showing: ") + "%d of %d items\n", filtered, total)
	fmt.Printf(colorize("yellow", "Scanned: ") + "%s\n\n", time.Now().Format("2006-01-02 15:04:05"))
}

func printStatistics(colorize func(string, string) string, stats MediaStats) {
	fmt.Printf(colorize("bold", "📊 COLLECTION STATISTICS\n"))
	fmt.Printf("  Total Items:     %s\n", colorize("white", fmt.Sprintf("%d", stats.TotalItems)))
	fmt.Printf("  Movies:          %s\n", colorize("blue", fmt.Sprintf("%d", stats.Movies)))
	fmt.Printf("  TV Episodes:     %s\n", colorize("purple", fmt.Sprintf("%d", stats.Episodes)))
	fmt.Printf("  Unique Shows:    %s\n", colorize("purple", fmt.Sprintf("%d", stats.UniqueShows)))
	fmt.Printf("  Available:       %s\n", colorize("green", fmt.Sprintf("%d", stats.OnDisk)))
	fmt.Printf("  Wanted:          %s\n", colorize("yellow", fmt.Sprintf("%d", stats.NotOnDisk)))
	fmt.Printf("  Downloading:     %s\n", colorize("cyan", fmt.Sprintf("%d", stats.Downloading)))
	
	if stats.TotalItems > 0 {
		availablePercent := float64(stats.OnDisk) / float64(stats.TotalItems) * 100
		fmt.Printf("  Completion:      %s\n", colorize("green", fmt.Sprintf("%.1f%%", availablePercent)))
	}
	fmt.Println()
}

func printMediaCollection(colorize func(string, string) string, media []*models.Media, showNZBs bool, store *bolthold.Store) {
	fmt.Printf(colorize("bold", "📺 MEDIA COLLECTION\n"))
	
	for i, item := range media {
		printMediaItem(colorize, item, i+1)
		
		if showNZBs {
			printNZBInfo(colorize, item.Trakt, store)
		}
		
		if i < len(media)-1 {
			fmt.Println(colorize("yellow", "────────────────────────────────────────────────────────────────────────────────"))
		}
	}
}

func printMediaItem(colorize func(string, string) string, item *models.Media, index int) {
	// Status indicator
	statusColor := "yellow"
	statusText := "WANTED"
	statusIcon := "⏳"
	
	if item.OnDisk {
		statusColor = "green"
		statusText = "AVAILABLE"
		statusIcon = "✅"
	} else if item.DownloadID > 0 {
		statusColor = "cyan"
		statusText = "DOWNLOADING"
		statusIcon = "⬇️"
	}
	
	// Type indicator
	typeIcon := "🎬" // Movie
	typeColor := "blue"
	typeText := "MOVIE"
	
	if item.IsEpisode() {
		typeIcon = "📺"
		typeColor = "purple"
		typeText = fmt.Sprintf("S%02dE%02d", item.Season, item.Number)
	}
	
	// Print main info
	fmt.Printf("%s %s %s %s\n",
		colorize("white", fmt.Sprintf("[%03d]", index)),
		typeIcon,
		colorize("bold", item.Title),
		colorize(statusColor, fmt.Sprintf("[%s %s]", statusIcon, statusText)))
	
	// Print details
	details := []string{}
	
	if item.Year > 0 {
		details = append(details, colorize("yellow", fmt.Sprintf("Year: %d", item.Year)))
	}
	
	if item.IsEpisode() {
		details = append(details, colorize(typeColor, typeText))
	} else {
		details = append(details, colorize(typeColor, typeText))
	}
	
	if item.IMDB != "" {
		details = append(details, colorize("white", fmt.Sprintf("IMDB: %s", item.IMDB)))
	}
	
	if item.DownloadID > 0 {
		details = append(details, colorize("cyan", fmt.Sprintf("Download ID: %d", item.DownloadID)))
	}
	
	if len(details) > 0 {
		fmt.Printf("    %s\n", strings.Join(details, " • "))
	}
	
	// File path
	if item.OnDisk && item.File != "" {
		fmt.Printf("    %s %s\n", colorize("green", "📁"), filepath.Base(item.File))
	}
	
	// Timestamps
	fmt.Printf("    %s %s • %s %s\n",
		colorize("white", "Created:"), item.CreatedAt.Format("2006-01-02 15:04"),
		colorize("white", "Updated:"), item.UpdatedAt.Format("2006-01-02 15:04"))
	
	fmt.Println()
}

func printNZBInfo(colorize func(string, string) string, traktID int64, store *bolthold.Store) {
	var nzbs []*models.NZB
	err := store.Find(&nzbs, bolthold.Where("Trakt").Eq(traktID))
	if err != nil || len(nzbs) == 0 {
		return
	}
	
	fmt.Printf("    %s NZB Files (%d found):\n", colorize("cyan", "💾"), len(nzbs))
	
	for i, nzb := range nzbs {
		statusIcon := "✅"
		statusColor := "green"
		if nzb.Failed {
			statusIcon = "❌"
			statusColor = "red"
		}
		
		sizeStr := formatBytes(nzb.Length)
		
		fmt.Printf("      %s %s %s %s\n",
			colorize(statusColor, statusIcon),
			colorize("white", fmt.Sprintf("[%d]", i+1)),
			colorize("white", nzb.Title),
			colorize("yellow", fmt.Sprintf("(%s)", sizeStr)))
	}
	fmt.Println()
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}