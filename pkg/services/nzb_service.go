package services

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
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
	repo           repository.Repository
	newsNabHost    string
	newsNabAPIKey  string
	blacklistFile  string
	blacklistCache map[string]struct{} // Use map for O(1) lookups
	blacklistMu    sync.RWMutex
}

func NewNZBService(repo repository.Repository, newsNabHost, newsNabAPIKey, blacklistFile string) *NZBService {
	return &NZBService{
		repo:           repo,
		newsNabHost:    newsNabHost,
		newsNabAPIKey:  newsNabAPIKey,
		blacklistFile:  blacklistFile,
		blacklistCache: make(map[string]struct{}),
	}
}

// GetNZBFromDB retrieves the best NZB for a given Trakt ID prioritizing remux first, then web-dl, and finally by size
func (s *NZBService) GetNZBFromDB(traktID int64) (*models.NZB, error) {
	return s.repo.GetNZB(traktID)
}

// PopulateNZB populates NZB entries for media not on disk with concurrent processing
func (s *NZBService) PopulateNZB() error {
	return s.PopulateNZBWithContext(context.Background())
}

// PopulateNZBWithContext populates NZB entries for media not on disk with concurrent processing and context
func (s *NZBService) PopulateNZBWithContext(ctx context.Context) error {

	return s.repo.ProcessMediaBatches(100, func(batch []*models.Media) error {
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

		return s.processBatchConcurrently(ctx, notOnDiskMedia, 5)
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

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			// Check context before processing
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			if err := s.populateNZBForMedia(m); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"trakt": m.Trakt,
					"title": m.Title,
				}).Error("Failed to populate NZB for media")
				errChan <- err
			}
		}(media)
	}

	wg.Wait()
	close(errChan)
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

	// Insert into database
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
		imdbID := media.ShowIMDBID
		if imdbID == "" {
			imdbID = media.IMDB
		}
		
		seasonPackResponse, seasonErr := s.searchSeasonPack(imdbID, media.Season)
		if seasonErr == nil {
			var seasonFeed newsnab.Feed
			if unmarshalErr := xml.Unmarshal([]byte(seasonPackResponse), &seasonFeed); unmarshalErr == nil {
				if len(seasonFeed.Channel.Items) > 0 {
					log.WithFields(log.Fields{
						"show":   media.ShowTitle,
						"season": media.Season,
						"count":  len(seasonFeed.Channel.Items),
					}).Info("Found season pack results")
					return seasonFeed, nil
				}
			}
		}
		
		log.WithFields(log.Fields{
			"show":    media.ShowTitle,
			"season":  media.Season,
			"episode": media.Number,
		}).Debug("No season pack found, searching for individual episode")
		
		xmlResponse, err = newsnab.SearchTVShow(imdbID, media.Season, media.Number, s.newsNabHost, s.newsNabAPIKey)
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

	if err := s.repo.SaveNZB(nzb); err != nil {
		return fmt.Errorf("saving NZB to database: %w", err)
	}

	return nil
}

// getBlacklist retrieves the blacklist with thread-safe caching
func (s *NZBService) getBlacklist() (map[string]struct{}, error) {
	s.blacklistMu.RLock()
	if len(s.blacklistCache) > 0 {
		// Return a copy to prevent external modifications
		cachedMap := make(map[string]struct{}, len(s.blacklistCache))
		for k := range s.blacklistCache {
			cachedMap[k] = struct{}{}
		}
		s.blacklistMu.RUnlock()
		return cachedMap, nil
	}
	s.blacklistMu.RUnlock()

	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()

	// Double-check after acquiring write lock
	if len(s.blacklistCache) > 0 {
		cachedMap := make(map[string]struct{}, len(s.blacklistCache))
		for k := range s.blacklistCache {
			cachedMap[k] = struct{}{}
		}
		return cachedMap, nil
	}

	blacklistWords, err := s.readBlacklist()
	if err != nil {
		return nil, err
	}

	// Convert to map for O(1) lookups
	blacklistMap := make(map[string]struct{}, len(blacklistWords))
	for _, word := range blacklistWords {
		blacklistMap[strings.ToLower(word)] = struct{}{}
	}

	s.blacklistCache = blacklistMap

	// Return a copy
	result := make(map[string]struct{}, len(blacklistMap))
	for k := range blacklistMap {
		result[k] = struct{}{}
	}
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

// searchSeasonPack searches for season packs for a TV show
func (s *NZBService) searchSeasonPack(imdbID string, season int64) (string, error) {
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=tvsearch&imdbid=%s&season=%d", 
		s.newsNabHost, s.newsNabAPIKey, imdbID, season)
	
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("searching season pack: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("season pack search returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading season pack response: %w", err)
	}
	
	responseStr := string(body)
	if s.containsSeasonPackIndicators(responseStr) {
		return responseStr, nil
	}
	
	return "", fmt.Errorf("no season packs found")
}

// containsSeasonPackIndicators checks if the response contains season pack results
func (s *NZBService) containsSeasonPackIndicators(xmlResponse string) bool {
	lowerResponse := strings.ToLower(xmlResponse)
	indicators := []string{
		"complete",
		"season",
		fmt.Sprintf("s%02d", 1),
	}
	
	for _, indicator := range indicators {
		if strings.Contains(lowerResponse, indicator) {
			return true
		}
	}
	return false
}

// isBlacklisted checks if a title is blacklisted using optimized map lookup
func (s *NZBService) isBlacklisted(title string, blacklist map[string]struct{}) bool {
	// Convert to lowercase once at the beginning
	titleLower := strings.ToLower(title)
	
	// Check each word in the blacklist map
	for word := range blacklist {
		// word is already lowercase from when we built the blacklist
		if strings.Contains(titleLower, word) {
			return true
		}
	}
	return false
}

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
