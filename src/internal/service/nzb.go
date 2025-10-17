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

var (
	seasonNotationPattern = regexp.MustCompile(`(?i)S\d{1,2}|Season[.\s]+\d+`)
	episodeMarkerPattern  = regexp.MustCompile(`(?i)E\d{1,2}`)
)

const (
	guidPrefix = "https://v2.nzbs.in/releases/"
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
	nzbs, err := s.nzbRepo.FindByTraktID(ctx, traktID, "", false)
	if err != nil {
		return nil, fmt.Errorf("querying nzbs: %w", err)
	}

	if len(nzbs) == 0 {
		return nil, fmt.Errorf("no nzb found for traktID %d: %w", traktID, domain.ErrNoNZBFound)
	}

	best := findBestScoredNZB(nzbs)
	s.logBestNZBSelected(traktID, best)
	return best, nil
}

func findBestScoredNZB(nzbs []domain.NZB) *domain.NZB {
	if len(nzbs) == 0 {
		return nil
	}

	best := &nzbs[0]
	for i := 1; i < len(nzbs); i++ {
		if nzbs[i].TotalScore > best.TotalScore {
			best = &nzbs[i]
		}
	}
	return best
}

func (s *NZBService) PopulateNZBs(ctx context.Context) error {
	medias, err := s.mediaRepo.FindNotOnDisk(ctx)
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	for _, media := range medias {
		if err := s.processMediaForNZB(ctx, &media); err != nil {
			s.logPopulationError(&media, err)
		}
	}
	return nil
}

func (s *NZBService) logPopulationError(media *domain.Media, err error) {
	log.WithFields(log.Fields{
		"error":   err,
		"traktID": media.TraktID,
	}).Error("failed to process media for nzb search")
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

	if results := s.trySeasonPackSearch(ctx, media); len(results) > 0 {
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
	hasSeasonNotation := seasonNotationPattern.MatchString(title)
	hasEpisodeMarker := episodeMarkerPattern.MatchString(title)
	return hasSeasonNotation && !hasEpisodeMarker
}

func isEpisode(media *domain.Media) bool {
	return media.Number > 0 && media.Season > 0
}

func (s *NZBService) insertResults(ctx context.Context, media *domain.Media, results []domain.SearchResult) error {
	blacklist := s.loadBlacklist()
	inserted := 0

	for _, result := range results {
		if err := s.insertValidatedResult(ctx, media, &result, blacklist); err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"title":   result.Title,
				"traktID": media.TraktID,
			}).Debug("failed to insert result")
			continue
		}
		inserted++
	}

	s.logInsertionSummary(media, len(results), inserted)
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

func (s *NZBService) insertValidatedResult(ctx context.Context, media *domain.Media, result *domain.SearchResult, blacklist []string) error {
	if isBlacklisted(result.Title, blacklist) {
		return fmt.Errorf("blacklisted")
	}

	parsed, err := parseSearchResult(result)
	if err != nil {
		return fmt.Errorf("parsing failed: %w", err)
	}

	valid, validationScore := validateParsedNZB(parsed, media, s.cfg)
	if !valid {
		return fmt.Errorf("validation failed: score %d", validationScore)
	}

	qualityScore := scoreQuality(parsed)
	if qualityScore < s.cfg.MinQualityScore {
		return fmt.Errorf("quality too low: score %d", qualityScore)
	}

	totalScore := validationScore + qualityScore
	if totalScore < s.cfg.MinTotalScore {
		return fmt.Errorf("total score too low: %d", totalScore)
	}

	return s.insertScoredNZB(ctx, media.TraktID, result, parsed, validationScore, qualityScore, totalScore)
}

func (s *NZBService) insertScoredNZB(ctx context.Context, traktID int64, result *domain.SearchResult, parsed *ParsedNZB, validationScore, qualityScore, totalScore int) error {
	nzb := buildNZBFromParsed(traktID, result, parsed, validationScore, qualityScore, totalScore)
	key := generateNZBKey(result.Title)

	err := s.nzbRepo.Insert(ctx, key, nzb)
	if err != nil && err != domain.ErrDuplicateKey {
		return fmt.Errorf("inserting nzb: %w", err)
	}
	return nil
}

func buildNZBFromParsed(traktID int64, result *domain.SearchResult, parsed *ParsedNZB, validationScore, qualityScore, totalScore int) *domain.NZB {
	return &domain.NZB{
		TraktID:         traktID,
		Link:            result.Link,
		Length:          result.Length,
		Title:           result.Title,
		Failed:          false,
		ParsedTitle:     parsed.Title,
		ParsedYear:      parsed.Year,
		ParsedSeason:    parsed.Season,
		ParsedEpisode:   parsed.Episode,
		Resolution:      parsed.Resolution,
		Source:          parsed.Source,
		Codec:           parsed.Codec,
		ValidationScore: validationScore,
		QualityScore:    qualityScore,
		TotalScore:      totalScore,
	}
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

func (s *NZBService) logInsertionSummary(media *domain.Media, total, inserted int) {
	log.WithFields(log.Fields{
		"traktID":  media.TraktID,
		"title":    media.Title,
		"total":    total,
		"inserted": inserted,
		"rejected": total - inserted,
	}).Info("nzb validation and insertion complete")
}

func (s *NZBService) logBestNZBSelected(traktID int64, nzb *domain.NZB) {
	log.WithFields(log.Fields{
		"traktID":         traktID,
		"title":           nzb.Title,
		"resolution":      nzb.Resolution,
		"source":          nzb.Source,
		"codec":           nzb.Codec,
		"validationScore": nzb.ValidationScore,
		"qualityScore":    nzb.QualityScore,
		"totalScore":      nzb.TotalScore,
	}).Debug("selected best scored nzb")
}
