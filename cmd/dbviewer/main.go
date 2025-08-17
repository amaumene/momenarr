
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

const (

	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"


	dbFileMode = 0600


	maxLineWidth    = 80
	separatorChar   = "─"
	headerBoxChar   = "═"
	completedIcon   = "✅"
	downloadingIcon = "⬇️"
	wantedIcon      = "⏳"
	movieIcon       = "🎬"
	episodeIcon     = "📺"
	frenchFlagIcon  = "🇫🇷"
	folderIcon      = "📁"
	statsIcon       = "📊"
)


type MediaStats struct {
	TotalItems  int
	Movies      int
	Episodes    int
	OnDisk      int
	NotOnDisk   int
	Downloading int
	UniqueShows int
}


type flags struct {
	dbPath     *string
	showStats  *bool
	showMovies *bool
	showShows  *bool
	onDiskOnly *bool
	noColor    *bool
	detailed   *bool
	sortBy     *string
}

func main() {
	flags := parseFlags()

	validateFlags(flags)

	store := openDatabaseStore(flags.dbPath)
	defer store.Close()

	mediaItems := fetchAllMedia(store)

	processAndDisplay(flags, store, mediaItems)
}


func parseFlags() *flags {
	return &flags{
		dbPath:     flag.String("db", "", "Path to the database file (required)"),
		showStats:  flag.Bool("stats", false, "Show only statistics"),
		showMovies: flag.Bool("movies", false, "Show only movies"),
		showShows:  flag.Bool("shows", false, "Show only TV shows"),
		onDiskOnly: flag.Bool("ondisk", false, "Show only media on disk"),
		noColor:    flag.Bool("no-color", false, "Disable colored output"),
		detailed:   flag.Bool("detailed", false, "Show detailed information including Trakt ID"),
		sortBy:     flag.String("sort", "title", "Sort by: title, year, created, updated, status"),
	}
}


func validateFlags(f *flags) {
	flag.Parse()

	if *f.dbPath == "" {
		printUsage()
		os.Exit(1)
	}

	if _, err := os.Stat(*f.dbPath); os.IsNotExist(err) {
		exitWithError("Database file '%s' does not exist", *f.dbPath)
	}
}


func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s -db <database-path> [options]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExample:\n")
	fmt.Fprintf(os.Stderr, "  %s -db /path/to/data.db -stats\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -db /path/to/data.db -movies -ondisk\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -db /path/to/data.db -detailed\n", os.Args[0])
}


func openDatabaseStore(dbPath *string) *bolthold.Store {
	store, err := bolthold.Open(*dbPath, dbFileMode, &bolthold.Options{})
	if err != nil {
		exitWithError("Error opening database: %v", err)
	}
	return store
}


func fetchAllMedia(store *bolthold.Store) []*models.Media {
	var mediaItems []*models.Media
	if err := store.Find(&mediaItems, nil); err != nil {
		exitWithError("Error reading media from database: %v", err)
	}
	return mediaItems
}


func processAndDisplay(f *flags, store *bolthold.Store, mediaItems []*models.Media) {
	filteredMedia := filterMedia(mediaItems, *f.showMovies, *f.showShows, *f.onDiskOnly)
	sortMedia(filteredMedia, *f.sortBy)
	stats := calculateStats(mediaItems)
	colorize := getColorizer(*f.noColor)

	printHeader(colorize, *f.dbPath, len(filteredMedia), len(mediaItems))

	if *f.showStats {
		printStatistics(colorize, stats)
		return
	}

	printMediaCollection(colorize, filteredMedia, store, *f.detailed)
	fmt.Print("\n" + colorize("cyan", "=== SUMMARY ===") + "\n")
	printStatistics(colorize, stats)
}

func filterMedia(media []*models.Media, moviesOnly, showsOnly, onDiskOnly bool) []*models.Media {
	var filtered []*models.Media

	for _, item := range media {

		if moviesOnly && !item.IsMovie() {
			continue
		}
		if showsOnly && !item.IsEpisode() {
			continue
		}


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
		updateMediaTypeStats(&stats, item, shows)
		updateDiskStats(&stats, item)
		updateDownloadStats(&stats, item)
	}

	stats.UniqueShows = len(shows)
	return stats
}


func updateMediaTypeStats(stats *MediaStats, item *models.Media, shows map[string]bool) {
	if item.IsMovie() {
		stats.Movies++
	} else {
		stats.Episodes++
		showTitle := extractShowTitle(item.Title)
		shows[showTitle] = true
	}
}


func extractShowTitle(title string) string {
	if strings.Contains(title, " - ") {
		parts := strings.Split(title, " - ")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return title
}


func updateDiskStats(stats *MediaStats, item *models.Media) {
	if item.OnDisk {
		stats.OnDisk++
	} else {
		stats.NotOnDisk++
	}
}


func updateDownloadStats(stats *MediaStats, item *models.Media) {
	if item.DownloadID > 0 {
		stats.Downloading++
	}
}

func getColorizer(noColor bool) func(string, string) string {
	if noColor {
		return func(color, text string) string { return text }
	}

	colors := map[string]string{
		"red":    colorRed,
		"green":  colorGreen,
		"yellow": colorYellow,
		"blue":   colorBlue,
		"purple": colorPurple,
		"cyan":   colorCyan,
		"white":  colorWhite,
		"bold":   colorBold,
	}

	return func(color, text string) string {
		if c, ok := colors[color]; ok {
			return c + text + colorReset
		}
		return text
	}
}

func printHeader(colorize func(string, string) string, dbPath string, filtered, total int) {
	boxTop := "╔" + strings.Repeat(headerBoxChar, maxLineWidth-2) + "╗\n"
	boxBottom := "╚" + strings.Repeat(headerBoxChar, maxLineWidth-2) + "╝\n"
	title := "MOMENARR DATABASE VIEWER"
	padding := (maxLineWidth - 2 - len(title)) / 2
	paddedTitle := strings.Repeat(" ", padding) + title + strings.Repeat(" ", maxLineWidth-2-len(title)-padding)

	fmt.Print(colorize("bold", boxTop))
	fmt.Print(colorize("bold", "║") + colorize("cyan", paddedTitle) + colorize("bold", "║\n"))
	fmt.Print(colorize("bold", boxBottom))
	fmt.Printf(colorize("yellow", "Database: ")+"%s\n", filepath.Base(dbPath))
	fmt.Printf(colorize("yellow", "Showing: ")+"%d of %d items\n", filtered, total)
	fmt.Printf(colorize("yellow", "Scanned: ")+"%s\n\n", time.Now().Format("2006-01-02 15:04:05"))
}

func printStatistics(colorize func(string, string) string, stats MediaStats) {
	fmt.Print(colorize("bold", fmt.Sprintf("%s COLLECTION STATISTICS\n", statsIcon)))
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

func printMediaCollection(colorize func(string, string) string, media []*models.Media, store *bolthold.Store, detailed bool) {
	fmt.Print(colorize("bold", "📺 MEDIA COLLECTION\n"))

	for i, item := range media {
		printMediaItem(colorize, item, i+1, detailed)

		if i < len(media)-1 {
			fmt.Println(colorize("yellow", strings.Repeat(separatorChar, maxLineWidth)))
		}
	}
}

func printMediaItem(colorize func(string, string) string, item *models.Media, index int, detailed bool) {
	status := getMediaStatus(item)
	mediaType := getMediaType(item)

	printMainInfo(colorize, item, index, status, mediaType)
	printDetailsLine(colorize, item, mediaType, detailed)
	printAdditionalInfo(colorize, item)

	fmt.Println()
}


type mediaStatus struct {
	color string
	text  string
	icon  string
}


type mediaType struct {
	icon  string
	color string
	text  string
}


func getMediaStatus(item *models.Media) mediaStatus {
	if item.OnDisk {
		return mediaStatus{"green", "AVAILABLE", completedIcon}
	}
	if item.DownloadID > 0 {
		return mediaStatus{"cyan", "DOWNLOADING", downloadingIcon}
	}
	return mediaStatus{"yellow", "WANTED", wantedIcon}
}


func getMediaType(item *models.Media) mediaType {
	if item.IsEpisode() {
		return mediaType{
			icon:  episodeIcon,
			color: "purple",
			text:  fmt.Sprintf("S%02dE%02d", item.Season, item.Number),
		}
	}
	return mediaType{movieIcon, "blue", "MOVIE"}
}


func printMainInfo(colorize func(string, string) string, item *models.Media, index int, status mediaStatus, mediaType mediaType) {
	fmt.Printf("%s %s %s %s\n",
		colorize("white", fmt.Sprintf("[%03d]", index)),
		mediaType.icon,
		colorize("bold", item.Title),
		colorize(status.color, fmt.Sprintf("[%s %s]", status.icon, status.text)))
}


func printDetailsLine(colorize func(string, string) string, item *models.Media, mediaType mediaType, detailed bool) {
	details := buildDetails(colorize, item, mediaType, detailed)
	if len(details) > 0 {
		fmt.Printf("    %s\n", strings.Join(details, " • "))
	}
}


func buildDetails(colorize func(string, string) string, item *models.Media, mediaType mediaType, detailed bool) []string {
	var details []string

	addBasicDetails(&details, colorize, item, mediaType)
	addLanguageDetails(&details, colorize, item)

	if detailed && item.Trakt > 0 {
		details = append(details, colorize("purple", fmt.Sprintf("Trakt: %d", item.Trakt)))
	}

	if item.DownloadID > 0 {
		details = append(details, colorize("cyan", fmt.Sprintf("Download ID: %d", item.DownloadID)))
	}

	return details
}

func addBasicDetails(details *[]string, colorize func(string, string) string, item *models.Media, mediaType mediaType) {
	if item.Year > 0 {
		*details = append(*details, colorize("yellow", fmt.Sprintf("Year: %d", item.Year)))
	}

	*details = append(*details, colorize(mediaType.color, mediaType.text))

	if item.TMDBID > 0 {
		*details = append(*details, colorize("white", fmt.Sprintf("TMDB: %d", item.TMDBID)))
	}
}

func addLanguageDetails(details *[]string, colorize func(string, string) string, item *models.Media) {
	if item.OriginalLanguage != "" {
		*details = append(*details, colorize("blue", fmt.Sprintf("Lang: %s", item.OriginalLanguage)))
	}

	if item.FrenchTitle != "" && item.FrenchTitle != item.Title {
		*details = append(*details, colorize("green", fmt.Sprintf("FR: %s", item.FrenchTitle)))
	}
}


func printAdditionalInfo(colorize func(string, string) string, item *models.Media) {
	if item.FrenchTitle != "" && item.FrenchTitle != item.Title {
		fmt.Printf("    %s %s\n", colorize("green", frenchFlagIcon), colorize("green", item.FrenchTitle))
	}

	if item.OnDisk && item.File != "" {
		fmt.Printf("    %s %s\n", colorize("green", folderIcon), filepath.Base(item.File))
	}

	fmt.Printf("    %s %s • %s %s\n",
		colorize("white", "Created:"), item.CreatedAt.Format("2006-01-02 15:04"),
		colorize("white", "Updated:"), item.UpdatedAt.Format("2006-01-02 15:04"))
}


func exitWithError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
