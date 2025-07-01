package services

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amaumene/momenarr/newsnab"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

// NZBService handles NZB search and management operations
type NZBService struct {
	repo         repository.Repository
	newsNabHost  string
	newsNabAPIKey string
	blacklistFile string
	blacklistCache []string
	blacklistCacheTime time.Time
	blacklistMu sync.RWMutex
	testMode     bool // When true, skips database storage
}

// NewNZBService creates a new NZBService
func NewNZBService(repo repository.Repository, newsNabHost, newsNabAPIKey, blacklistFile string, testMode bool) *NZBService {
	return &NZBService{
		repo:          repo,
		newsNabHost:   newsNabHost,
		newsNabAPIKey: newsNabAPIKey,
		blacklistFile: blacklistFile,
		testMode:      testMode,
	}
}

// Resolution represents different resolution priorities for NZB selection
type Resolution int

const (
	Resolution2160p Resolution = iota // 4K - highest priority
	Resolution1080p                   // 1080p - second priority
	Resolution720p                    // 720p - third priority
	ResolutionOther                   // Any other resolution - lowest priority
)

// Quality represents different quality preferences for NZB selection
type Quality int

const (
	QualityRemux Quality = iota
	QualityWebDL
	QualityAny
)

var (
	// Resolution regex patterns
	resolution2160pRegex = regexp.MustCompile("(?i)2160p|4k")
	resolution1080pRegex = regexp.MustCompile("(?i)1080p")
	resolution720pRegex  = regexp.MustCompile("(?i)720p")
	
	// Quality regex patterns
	remuxRegex = regexp.MustCompile("(?i)remux")
	webDLRegex = regexp.MustCompile("(?i)web-dl")
	
	blacklistCacheDuration = 5 * time.Minute
)

// GetNZBFromDB retrieves the best NZB for a given Trakt ID prioritizing resolution then file size
func (s *NZBService) GetNZBFromDB(traktID int64) (*models.NZB, error) {
	allNZBs, err := s.repo.FindNZBsByTraktIDs([]int64{traktID})
	if err != nil {
		return nil, fmt.Errorf("finding NZBs: %w", err)
	}

	// Group NZBs by resolution, keeping only the biggest file size for each resolution
	var candidates = map[Resolution]*models.NZB{
		Resolution2160p: nil,
		Resolution1080p: nil,
		Resolution720p:  nil,
		ResolutionOther: nil,
	}

	for _, nzb := range allNZBs {
		if nzb.Failed || nzb.Trakt != traktID {
			continue
		}

		// Determine resolution priority
		resolution := s.getResolution(nzb.Title)
		
		// Keep the biggest file for each resolution
		if candidates[resolution] == nil || nzb.Length > candidates[resolution].Length {
			candidates[resolution] = nzb
		}
	}

	// Return highest priority resolution available (2160p > 1080p > 720p > other)
	resolutionPriority := []Resolution{Resolution2160p, Resolution1080p, Resolution720p, ResolutionOther}
	for _, resolution := range resolutionPriority {
		if candidates[resolution] != nil {
			log.WithFields(log.Fields{
				"trakt": traktID,
				"title": candidates[resolution].Title,
				"size_gb": float64(candidates[resolution].Length) / (1024 * 1024 * 1024),
				"resolution": s.getResolutionName(resolution),
			}).Debug("Selected NZB by resolution and size")
			return candidates[resolution], nil
		}
	}

	return nil, fmt.Errorf("no NZB found for Trakt ID %d", traktID)
}

// getResolution determines the resolution priority of an NZB title
func (s *NZBService) getResolution(title string) Resolution {
	if resolution2160pRegex.MatchString(title) {
		return Resolution2160p
	}
	if resolution1080pRegex.MatchString(title) {
		return Resolution1080p
	}
	if resolution720pRegex.MatchString(title) {
		return Resolution720p
	}
	return ResolutionOther
}

// getResolutionName returns a human-readable name for a resolution
func (s *NZBService) getResolutionName(resolution Resolution) string {
	switch resolution {
	case Resolution2160p:
		return "2160p/4K"
	case Resolution1080p:
		return "1080p"
	case Resolution720p:
		return "720p"
	case ResolutionOther:
		return "Other"
	default:
		return "Unknown"
	}
}

// findNZBByQuality finds NZB by quality preference (deprecated - use GetNZBFromDB instead)
// This function is kept for backward compatibility but is no longer used internally

// matchesQuality checks if the title matches the quality preference
func (s *NZBService) matchesQuality(title string, quality Quality) bool {
	switch quality {
	case QualityRemux:
		return remuxRegex.MatchString(title)
	case QualityWebDL:
		return webDLRegex.MatchString(title)
	case QualityAny:
		return true
	}
	return false
}

// PopulateNZB populates NZB entries for media not on disk with concurrent processing
func (s *NZBService) PopulateNZB() error {
	return s.PopulateNZBWithContext(context.Background())
}

// PopulateNZBWithContext populates NZB entries for media not on disk with concurrent processing and context
func (s *NZBService) PopulateNZBWithContext(ctx context.Context) error {
	// Process media in batches to manage memory usage
	return s.repo.ProcessMediaBatches(50, func(batch []*models.Media) error {
		// Filter for media not on disk
		var notOnDiskMedia []*models.Media
		for _, media := range batch {
			if !media.OnDisk {
				notOnDiskMedia = append(notOnDiskMedia, media)
			}
		}

		if len(notOnDiskMedia) == 0 {
			return nil
		}

		log.WithField("count", len(notOnDiskMedia)).Info("Processing NZB batch for media not on disk")

		// Process batch with controlled concurrency
		return s.processBatchConcurrently(ctx, notOnDiskMedia, 5) // Limit to 5 concurrent operations
	})
}

// processBatchConcurrently processes a batch of media with controlled concurrency
func (s *NZBService) processBatchConcurrently(ctx context.Context, medias []*models.Media, maxConcurrency int) error {
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	errChan := make(chan error, len(medias))

	for _, media := range medias {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(m *models.Media) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := s.populateNZBForMedia(m); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"trakt": m.Trakt,
					"title": m.Title,
				}).Error("Failed to populate NZB for media")
				errChan <- err
			}
		}(media)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors (optional - could choose to continue on errors)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		log.WithField("error_count", len(errors)).Warn("Some NZB population operations failed")
	}

	return nil
}

// populateNZBForMedia populates NZB entries for a specific media item
func (s *NZBService) populateNZBForMedia(media *models.Media) error {
	feed, err := s.searchNZB(media)
	if err != nil {
		return fmt.Errorf("searching NZB for media %d: %w", media.Trakt, err)
	}

	if len(feed.Channel.Items) == 0 {
		log.WithFields(log.Fields{
			"trakt": media.Trakt,
			"title": media.Title,
		}).Warn("No NZB items found for media")
		return nil
	}

	if err := s.insertNZBItems(media, feed.Channel.Items); err != nil {
		return fmt.Errorf("inserting NZB items: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt": media.Trakt,
		"title": media.Title,
		"count": len(feed.Channel.Items),
	}).Debug("Successfully populated NZB items for media")

	return nil
}

// searchNZB searches for NZB items based on media type
func (s *NZBService) searchNZB(media *models.Media) (newsnab.Feed, error) {
	var feed newsnab.Feed
	var xmlResponse string
	var err error

	if media.IsEpisode() {
		xmlResponse, err = newsnab.SearchTVShow(media.IMDB, media.Season, media.Number, s.newsNabHost, s.newsNabAPIKey)
		if err != nil {
			return feed, fmt.Errorf("searching NZB for episode: %w", err)
		}
	} else {
		xmlResponse, err = newsnab.SearchMovie(media.IMDB, s.newsNabHost, s.newsNabAPIKey)
		if err != nil {
			return feed, fmt.Errorf("searching NZB for movie: %w", err)
		}
	}

	if err := xml.Unmarshal([]byte(xmlResponse), &feed); err != nil {
		return feed, fmt.Errorf("unmarshalling XML NZB response: %w", err)
	}

	return feed, nil
}

// insertNZBItems inserts NZB items into the database after blacklist filtering
func (s *NZBService) insertNZBItems(media *models.Media, items []newsnab.Item) error {
	blacklist, err := s.getBlacklist()
	if err != nil {
		return fmt.Errorf("getting blacklist: %w", err)
	}

	for _, item := range items {
		if s.isBlacklisted(item.Title, blacklist) {
			log.WithFields(log.Fields{
				"title": item.Title,
				"trakt": media.Trakt,
			}).Debug("Skipping blacklisted NZB item")
			continue
		}

		if err := s.insertNZBItem(media, item); err != nil {
			log.WithError(err).WithField("title", item.Title).Error("Failed to insert NZB item")
			continue
		}
	}

	return nil
}

// insertNZBItem inserts a single NZB item into the database
func (s *NZBService) insertNZBItem(media *models.Media, item newsnab.Item) error {
	length, err := strconv.ParseInt(item.Enclosure.Length, 10, 64)
	if err != nil {
		return fmt.Errorf("converting NZB length to int64: %w", err)
	}

	nzb := &models.NZB{
		Trakt:     media.Trakt,
		Link:      item.Enclosure.URL,
		Length:    length,
		Title:     item.Title,
		Failed:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if s.testMode {
		// Test mode: skip saving to database, no output here - only selected NZBs are output during download phase
		return nil
	}

	if err := s.repo.SaveNZB(nzb); err != nil {
		// Handle duplicate key error gracefully
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("saving NZB to database: %w", err)
		}
	}

	return nil
}

// getBlacklist retrieves the blacklist with thread-safe caching
func (s *NZBService) getBlacklist() ([]string, error) {
	// Check if cache is still valid (read lock)
	s.blacklistMu.RLock()
	if time.Since(s.blacklistCacheTime) < blacklistCacheDuration && len(s.blacklistCache) > 0 {
		cachedList := make([]string, len(s.blacklistCache))
		copy(cachedList, s.blacklistCache)
		s.blacklistMu.RUnlock()
		return cachedList, nil
	}
	s.blacklistMu.RUnlock()

	// Need to refresh cache (write lock)
	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()

	// Double-check pattern - another goroutine might have updated the cache
	if time.Since(s.blacklistCacheTime) < blacklistCacheDuration && len(s.blacklistCache) > 0 {
		cachedList := make([]string, len(s.blacklistCache))
		copy(cachedList, s.blacklistCache)
		return cachedList, nil
	}

	// Read fresh blacklist
	blacklist, err := s.readBlacklist()
	if err != nil {
		return nil, err
	}

	// Update cache
	s.blacklistCache = blacklist
	s.blacklistCacheTime = time.Now()

	// Return copy to avoid concurrent access issues
	result := make([]string, len(blacklist))
	copy(result, blacklist)
	return result, nil
}

// readBlacklist reads the blacklist file
func (s *NZBService) readBlacklist() ([]string, error) {
	file, err := os.Open(s.blacklistFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.WithField("file", s.blacklistFile).Debug("Blacklist file does not exist, using empty blacklist")
			return []string{}, nil
		}
		return nil, fmt.Errorf("opening blacklist file: %w", err)
	}
	defer file.Close()

	var blacklist []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			blacklist = append(blacklist, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning blacklist file: %w", err)
	}

	log.WithFields(log.Fields{
		"file":  s.blacklistFile,
		"count": len(blacklist),
	}).Debug("Loaded blacklist")

	return blacklist, nil
}

// isBlacklisted checks if a title is blacklisted
func (s *NZBService) isBlacklisted(title string, blacklist []string) bool {
	titleLower := strings.ToLower(title)
	for _, word := range blacklist {
		if strings.Contains(titleLower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

// MarkNZBFailed marks an NZB as failed
func (s *NZBService) MarkNZBFailed(traktID int64) error {
	nzb, err := s.repo.GetNZB(traktID)
	if err != nil {
		return fmt.Errorf("getting NZB: %w", err)
	}

	nzb.MarkFailed()
	if err := s.repo.SaveNZB(nzb); err != nil {
		return fmt.Errorf("marking NZB as failed: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt": traktID,
		"title": nzb.Title,
	}).Info("Marked NZB as failed")

	return nil
}

// TestResolutionDetection is a helper function to test resolution detection
func (s *NZBService) TestResolutionDetection(titles []string) {
	log.Info("Testing resolution detection:")
	for _, title := range titles {
		resolution := s.getResolution(title)
		log.WithFields(log.Fields{
			"title": title,
			"detected_resolution": s.getResolutionName(resolution),
		}).Info("Resolution test")
	}
}