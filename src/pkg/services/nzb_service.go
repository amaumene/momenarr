package services

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
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
}

// NewNZBService creates a new NZBService
func NewNZBService(repo repository.Repository, newsNabHost, newsNabAPIKey, blacklistFile string) *NZBService {
	return &NZBService{
		repo:          repo,
		newsNabHost:   newsNabHost,
		newsNabAPIKey: newsNabAPIKey,
		blacklistFile: blacklistFile,
	}
}

// Quality represents different quality preferences for NZB selection
type Quality int

const (
	QualityRemux Quality = iota
	QualityWebDL
	QualityAny
)

var (
	remuxRegex = regexp.MustCompile("(?i)remux")
	webDLRegex = regexp.MustCompile("(?i)web-dl")
	blacklistCacheDuration = 5 * time.Minute
)

// GetNZBFromDB retrieves the best NZB for a given Trakt ID
func (s *NZBService) GetNZBFromDB(traktID int64) (*models.NZB, error) {
	// Try to find Remux quality first
	nzb, err := s.findNZBByQuality(traktID, QualityRemux)
	if err == nil && nzb != nil {
		return nzb, nil
	}

	// Try Web-DL quality
	nzb, err = s.findNZBByQuality(traktID, QualityWebDL)
	if err == nil && nzb != nil {
		return nzb, nil
	}

	// Try any quality
	nzb, err = s.findNZBByQuality(traktID, QualityAny)
	if err == nil && nzb != nil {
		return nzb, nil
	}

	return nil, fmt.Errorf("no NZB found for Trakt ID %d", traktID)
}

// findNZBByQuality finds NZB by quality preference
func (s *NZBService) findNZBByQuality(traktID int64, quality Quality) (*models.NZB, error) {
	// Get all NZBs for this Trakt ID
	allNZBs, err := s.repo.FindNZBsByTraktIDs([]int64{traktID})
	if err != nil {
		return nil, fmt.Errorf("finding NZBs: %w", err)
	}

	// Filter by quality and select the best one
	var bestNZB *models.NZB
	for _, nzb := range allNZBs {
		if nzb.Failed || nzb.Trakt != traktID {
			continue
		}

		if !s.matchesQuality(nzb.Title, quality) {
			continue
		}

		if bestNZB == nil || nzb.Length > bestNZB.Length {
			bestNZB = nzb
		}
	}

	if bestNZB == nil {
		return nil, fmt.Errorf("no NZB found for quality %d", quality)
	}

	return bestNZB, nil
}

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

// PopulateNZB populates NZB entries for media not on disk
func (s *NZBService) PopulateNZB() error {
	medias, err := s.repo.FindMediaNotOnDisk()
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	log.WithField("count", len(medias)).Info("Populating NZBs for media not on disk")

	for _, media := range medias {
		if err := s.populateNZBForMedia(media); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"trakt": media.Trakt,
				"title": media.Title,
			}).Error("Failed to populate NZB for media")
			continue
		}
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

	if err := s.repo.SaveNZB(nzb); err != nil {
		// Handle duplicate key error gracefully
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("saving NZB to database: %w", err)
		}
	}

	return nil
}

// getBlacklist retrieves the blacklist with caching
func (s *NZBService) getBlacklist() ([]string, error) {
	// Check if cache is still valid
	if time.Since(s.blacklistCacheTime) < blacklistCacheDuration && len(s.blacklistCache) > 0 {
		return s.blacklistCache, nil
	}

	// Read fresh blacklist
	blacklist, err := s.readBlacklist()
	if err != nil {
		return nil, err
	}

	// Update cache
	s.blacklistCache = blacklist
	s.blacklistCacheTime = time.Now()

	return blacklist, nil
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