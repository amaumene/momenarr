package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	log "github.com/sirupsen/logrus"
)

const (
	regexRemux          = "(?i)remux"
	regexWebDL          = "(?i)web-dl"
	regexSeasonNotation = `(?i)S\d{1,2}|Season[.\s]+\d+`
	regexEpisodeMarker  = `(?i)E\d{1,2}`
	guidPrefix          = "https://v2.nzbs.in/releases/"
)

type NZBService struct {
	cfg       *config.Config
	mediaRepo domain.MediaRepository
	nzbRepo   domain.NZBRepository
	searcher  domain.NZBSearcher
}

func NewNZBService(cfg *config.Config, mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, searcher domain.NZBSearcher) *NZBService {
	return &NZBService{
		cfg:       cfg,
		mediaRepo: mediaRepo,
		nzbRepo:   nzbRepo,
		searcher:  searcher,
	}
}

func (s *NZBService) GetNZB(ctx context.Context, traktID int64) (*domain.NZB, error) {
	if nzb, err := s.findNZBByPattern(ctx, traktID, regexRemux); err == nil && nzb != nil {
		return nzb, nil
	}

	if nzb, err := s.findNZBByPattern(ctx, traktID, regexWebDL); err == nil && nzb != nil {
		return nzb, nil
	}

	if nzb, err := s.findNZBByPattern(ctx, traktID, ""); err == nil && nzb != nil {
		return nzb, nil
	}

	return nil, fmt.Errorf("no nzb found for traktID %d: %w", traktID, domain.ErrNoNZBFound)
}

func (s *NZBService) findNZBByPattern(ctx context.Context, traktID int64, pattern string) (*domain.NZB, error) {
	nzbs, err := s.nzbRepo.FindByTraktID(ctx, traktID, pattern, false)
	if err != nil || len(nzbs) == 0 {
		return nil, err
	}
	return &nzbs[0], nil
}

func (s *NZBService) PopulateNZBs(ctx context.Context) error {
	medias, err := s.mediaRepo.FindNotOnDisk(ctx)
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	for _, media := range medias {
		if err := s.processMediaForNZB(ctx, &media); err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"traktID": media.TraktID,
			}).Error("failed to process media for nzb search")
		}
	}
	return nil
}

func (s *NZBService) processMediaForNZB(ctx context.Context, media *domain.Media) error {
	results, err := s.searchNZB(ctx, media)
	if err != nil {
		return fmt.Errorf("searching nzb: %w", err)
	}

	if len(results) == 0 {
		s.logNoNZBFound(media)
		return nil
	}

	return s.insertResults(ctx, media, results)
}

func (s *NZBService) searchNZB(ctx context.Context, media *domain.Media) ([]domain.SearchResult, error) {
	if isEpisode(media) {
		return s.searchEpisodeWithSeasonPackPriority(ctx, media)
	}
	return s.searcher.SearchMovie(ctx, media.IMDB)
}

func (s *NZBService) searchEpisodeWithSeasonPackPriority(ctx context.Context, media *domain.Media) ([]domain.SearchResult, error) {
	s.logSearchStart(media)

	if results := s.trySeasonPackSearch(ctx, media); results != nil {
		return results, nil
	}

	s.logFallbackToEpisode(media)
	return s.searcher.SearchEpisode(ctx, media.IMDB, media.Season, media.Number)
}

func (s *NZBService) trySeasonPackSearch(ctx context.Context, media *domain.Media) []domain.SearchResult {
	seasonPackResults, err := s.searcher.SearchSeasonPack(ctx, media.IMDB, media.Season)
	if err != nil {
		s.logSeasonPackError(media, err)
		return nil
	}

	s.logSeasonPackResults(media, len(seasonPackResults))

	if len(seasonPackResults) == 0 {
		return nil
	}

	return s.filterAndLogSeasonPacks(media, seasonPackResults)
}

func (s *NZBService) filterAndLogSeasonPacks(media *domain.Media, results []domain.SearchResult) []domain.SearchResult {
	filtered := filterSeasonPacks(results)
	s.logFilterResults(media, len(results), len(filtered))

	if len(filtered) > 0 {
		s.logSeasonPacksFound(media, len(filtered))
		return filtered
	}
	return nil
}

func filterSeasonPacks(results []domain.SearchResult) []domain.SearchResult {
	filtered := make([]domain.SearchResult, 0, len(results))
	for _, result := range results {
		if isSeasonPackTitle(result.Title) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func isSeasonPackTitle(title string) bool {
	hasSeasonNotation, err := regexp.MatchString(regexSeasonNotation, title)
	if err != nil {
		return false
	}
	hasEpisodeMarker, err := regexp.MatchString(regexEpisodeMarker, title)
	if err != nil {
		return false
	}
	return hasSeasonNotation && !hasEpisodeMarker
}

func isEpisode(media *domain.Media) bool {
	return media.Number > 0 && media.Season > 0
}

func (s *NZBService) insertResults(ctx context.Context, media *domain.Media, results []domain.SearchResult) error {
	blacklist := s.loadBlacklist()

	for _, result := range results {
		if err := s.insertResult(ctx, media.TraktID, &result, blacklist); err != nil {
			return err
		}
	}
	return nil
}

func (s *NZBService) loadBlacklist() []string {
	file, err := os.Open(s.cfg.BlacklistPath())
	if err != nil {
		log.WithField("error", err).Warn("blacklist file not found, using empty blacklist")
		return []string{}
	}
	defer file.Close()

	return scanBlacklist(file)
}

func scanBlacklist(file *os.File) []string {
	var blacklist []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		blacklist = append(blacklist, scanner.Text())
	}
	return blacklist
}

func (s *NZBService) insertResult(ctx context.Context, traktID int64, result *domain.SearchResult, blacklist []string) error {
	if isBlacklisted(result.Title, blacklist) {
		return nil
	}

	nzb := &domain.NZB{
		TraktID: traktID,
		Link:    result.Link,
		Length:  result.Length,
		Title:   result.Title,
		Failed:  false,
	}

	key := generateNZBKey(result.Title)
	err := s.nzbRepo.Insert(ctx, key, nzb)
	if err != nil && err != domain.ErrDuplicateKey {
		return fmt.Errorf("inserting nzb: %w", err)
	}
	return nil
}

func isBlacklisted(title string, blacklist []string) bool {
	lowerTitle := strings.ToLower(title)
	for _, word := range blacklist {
		if strings.Contains(lowerTitle, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func generateNZBKey(title string) string {
	return strings.TrimPrefix(title, guidPrefix)
}

func (s *NZBService) logNoNZBFound(media *domain.Media) {
	log.WithFields(log.Fields{
		"traktID": media.TraktID,
		"title":   media.Title,
	}).Warn("no nzb results found for media")
}

func (s *NZBService) logSearchStart(media *domain.Media) {
	log.WithFields(log.Fields{
		"title":   media.Title,
		"season":  media.Season,
		"episode": media.Number,
		"imdb":    media.IMDB,
	}).Debug("searching for season pack first")
}

func (s *NZBService) logSeasonPackError(media *domain.Media, err error) {
	log.WithFields(log.Fields{
		"error": err,
		"title": media.Title,
	}).Warn("failed to search for season pack, falling back to episode search")
}

func (s *NZBService) logSeasonPackResults(media *domain.Media, count int) {
	log.WithFields(log.Fields{
		"title":        media.Title,
		"resultsCount": count,
	}).Debug("received season pack search results")
}

func (s *NZBService) logFilterResults(media *domain.Media, before, after int) {
	log.WithFields(log.Fields{
		"title":        media.Title,
		"beforeFilter": before,
		"afterFilter":  after,
	}).Debug("filtered season pack results")
}

func (s *NZBService) logSeasonPacksFound(media *domain.Media, count int) {
	log.WithFields(log.Fields{
		"title":       media.Title,
		"seasonPacks": count,
	}).Info("found season packs, using season pack instead of single episode")
}

func (s *NZBService) logFallbackToEpisode(media *domain.Media) {
	log.WithFields(log.Fields{
		"title":   media.Title,
		"season":  media.Season,
		"episode": media.Number,
	}).Debug("no season packs found, searching for single episode")
}
