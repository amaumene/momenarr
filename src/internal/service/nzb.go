package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	log "github.com/sirupsen/logrus"
)

var (
	seasonNotationPattern = regexp.MustCompile(`(?i)S\d{1,2}|Season[.\s]+\d+`)
	episodeMarkerPattern  = regexp.MustCompile(`(?i)E\d{1,2}`)
)

const (
	guidPrefix              = "https://v2.nzbs.in/releases/"
	topNZBCountForLogging   = 3
	minNZBListForComparison = 1
)

// NZBService handles NZB search, validation, and storage operations.
// It caches the blacklist for performance and coordinates between search providers and storage.
type NZBService struct {
	cfg            *config.Config
	mediaRepo      domain.MediaRepository
	nzbRepo        domain.NZBRepository
	searcher       domain.NZBSearcher
	blacklistCache []string
	blacklistMutex sync.RWMutex
	blacklistOnce  sync.Once
}

// NewNZBService creates a new NZBService with the provided dependencies.
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
	s.logBestNZBSelected(traktID, best, nzbs)
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

func findTopNZBs(nzbs []domain.NZB, count int) []domain.NZB {
	if len(nzbs) == 0 {
		return nil
	}
	if count > len(nzbs) {
		count = len(nzbs)
	}

	top := make([]domain.NZB, 0, count)
	for _, nzb := range nzbs {
		if len(top) < count {
			top = append(top, nzb)
			for i := len(top) - 1; i > 0 && top[i].TotalScore > top[i-1].TotalScore; i-- {
				top[i], top[i-1] = top[i-1], top[i]
			}
		} else if nzb.TotalScore > top[count-1].TotalScore {
			top[count-1] = nzb
			for i := count - 1; i > 0 && top[i].TotalScore > top[i-1].TotalScore; i-- {
				top[i], top[i-1] = top[i-1], top[i]
			}
		}
	}
	return top
}

func (s *NZBService) PopulateNZBs(ctx context.Context) error {
	medias, err := s.mediaRepo.FindNotOnDisk(ctx)
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	for _, media := range medias {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during NZB population: %w", err)
		}
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

	blacklist := s.loadBlacklist()
	results := s.trySeasonPackSearch(ctx, media)

	if len(results) > 0 {
		nonBlacklisted := s.filterBlacklisted(results, blacklist)
		if len(nonBlacklisted) > 0 {
			s.logSeasonPacksPassedBlacklist(media, len(results), len(nonBlacklisted))
			return nonBlacklisted, nil
		}
		s.logAllSeasonPacksBlacklisted(media, len(results))
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

func (s *NZBService) filterBlacklisted(results []domain.SearchResult, blacklist []string) []domain.SearchResult {
	if len(blacklist) == 0 {
		return results
	}

	filtered := make([]domain.SearchResult, 0, len(results))
	for _, result := range results {
		if !isBlacklisted(result.Title, blacklist) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func isEpisode(media *domain.Media) bool {
	return media.Number > 0 && media.Season > 0
}

func (s *NZBService) insertResults(ctx context.Context, media *domain.Media, results []domain.SearchResult) error {
	blacklist := s.loadBlacklist()
	inserted := 0

	for _, result := range results {
		if err := ctx.Err(); err != nil {
			log.WithField("inserted", inserted).Debug("context cancelled during NZB insertion")
			break
		}
		if err := s.insertValidatedResult(ctx, media, &result, blacklist); err != nil {
			continue
		}
		inserted++
	}

	s.logInsertionSummary(media, len(results), inserted)
	return nil
}

func (s *NZBService) loadBlacklist() []string {
	s.blacklistOnce.Do(func() {
		s.blacklistCache = s.readBlacklistFromFile()
	})

	s.blacklistMutex.RLock()
	defer s.blacklistMutex.RUnlock()
	return s.blacklistCache
}

func (s *NZBService) readBlacklistFromFile() []string {
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
		s.logRejectionBlacklisted(media, result, blacklist)
		return fmt.Errorf("nzb validation failed: release blacklisted")
	}

	parsed, err := parseSearchResult(result)
	if err != nil {
		s.logRejectionParseFailed(media, result, err)
		return fmt.Errorf("parsing failed: %w", err)
	}

	valid, validationScore := s.validateWithLogging(parsed, media)
	if !valid {
		return fmt.Errorf("nzb validation failed: validation score %d below minimum", validationScore)
	}

	qualityScore := scoreQuality(parsed)
	if qualityScore < s.cfg.MinQualityScore {
		s.logRejectionQualityTooLow(media, result, parsed, qualityScore)
		return fmt.Errorf("nzb validation failed: quality score %d below minimum %d", qualityScore, s.cfg.MinQualityScore)
	}

	totalScore := validationScore + qualityScore
	if totalScore < s.cfg.MinTotalScore {
		s.logRejectionTotalScoreTooLow(media, result, validationScore, qualityScore, totalScore)
		return fmt.Errorf("nzb validation failed: total score %d below minimum %d", totalScore, s.cfg.MinTotalScore)
	}

	s.logReleaseAccepted(media, result, parsed, validationScore, qualityScore, totalScore)
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

func (s *NZBService) validateWithLogging(parsed *ParsedNZB, media *domain.Media) (bool, int) {
	isEpisode := isMediaEpisode(media)

	titleValid, titleScore := validateTitle(parsed.Title, media.Title, s.cfg.TitleSimilarityMin)
	if !titleValid {
		similarity := calculateSimilarity(parsed.Title, normalizeTitle(media.Title))
		log.WithFields(log.Fields{
			"traktID":       media.TraktID,
			"mediaTitle":    media.Title,
			"parsedTitle":   parsed.Title,
			"similarity":    fmt.Sprintf("%.2f", similarity),
			"minSimilarity": fmt.Sprintf("%.2f", s.cfg.TitleSimilarityMin),
		}).Debug("release rejected: title similarity too low")
		return false, 0
	}

	yearValid, yearScore := validateYear(parsed.Year, media.Year, s.cfg.YearTolerance, isEpisode)
	if !yearValid {
		log.WithFields(log.Fields{
			"traktID":    media.TraktID,
			"mediaTitle": media.Title,
			"mediaYear":  media.Year,
			"parsedYear": parsed.Year,
			"isEpisode":  isEpisode,
		}).Debug("release rejected: year mismatch")
		return false, 0
	}

	seValid, seScore := validateSeasonEpisode(parsed, media)
	if !seValid {
		log.WithFields(log.Fields{
			"traktID":       media.TraktID,
			"mediaTitle":    media.Title,
			"mediaSeason":   media.Season,
			"mediaEpisode":  media.Number,
			"parsedSeason":  parsed.Season,
			"parsedEpisode": parsed.Episode,
		}).Debug("release rejected: season/episode mismatch")
		return false, 0
	}

	totalScore := titleScore + yearScore + seScore
	if totalScore < s.cfg.MinValidationScore {
		log.WithFields(log.Fields{
			"traktID":          media.TraktID,
			"mediaTitle":       media.Title,
			"titleScore":       titleScore,
			"yearScore":        yearScore,
			"seasonEpisodeScore": seScore,
			"validationScore":  totalScore,
			"minRequired":      s.cfg.MinValidationScore,
		}).Debug("release rejected: validation score too low")
		return false, totalScore
	}

	return true, totalScore
}

func (s *NZBService) logRejectionBlacklisted(media *domain.Media, result *domain.SearchResult, blacklist []string) {
	matchedWord := ""
	lowerTitle := strings.ToLower(result.Title)
	for _, word := range blacklist {
		if strings.Contains(lowerTitle, strings.ToLower(word)) {
			matchedWord = word
			break
		}
	}

	log.WithFields(log.Fields{
		"traktID":     media.TraktID,
		"mediaTitle":  media.Title,
		"nzbTitle":    result.Title,
		"matchedWord": matchedWord,
	}).Debug("release rejected: blacklisted")
}

func (s *NZBService) logRejectionParseFailed(media *domain.Media, result *domain.SearchResult, err error) {
	log.WithFields(log.Fields{
		"traktID":    media.TraktID,
		"mediaTitle": media.Title,
		"nzbTitle":   result.Title,
		"error":      err,
	}).Debug("release rejected: parse failed")
}

func (s *NZBService) logRejectionQualityTooLow(media *domain.Media, result *domain.SearchResult, parsed *ParsedNZB, qualityScore int) {
	resScore := scoreResolution(parsed.Resolution)
	srcScore := scoreSource(parsed.Source)
	codecScore := scoreCodec(parsed.Codec)
	flagsScore := scoreFlags(parsed.Proper, parsed.Repack)

	log.WithFields(log.Fields{
		"traktID":         media.TraktID,
		"mediaTitle":      media.Title,
		"nzbTitle":        result.Title,
		"resolution":      parsed.Resolution,
		"source":          parsed.Source,
		"codec":           parsed.Codec,
		"resolutionScore": resScore,
		"sourceScore":     srcScore,
		"codecScore":      codecScore,
		"flagsScore":      flagsScore,
		"qualityScore":    qualityScore,
		"minRequired":     s.cfg.MinQualityScore,
	}).Debug("release rejected: quality score too low")
}

func (s *NZBService) logRejectionTotalScoreTooLow(media *domain.Media, result *domain.SearchResult, validationScore, qualityScore, totalScore int) {
	log.WithFields(log.Fields{
		"traktID":         media.TraktID,
		"mediaTitle":      media.Title,
		"nzbTitle":        result.Title,
		"validationScore": validationScore,
		"qualityScore":    qualityScore,
		"totalScore":      totalScore,
		"minRequired":     s.cfg.MinTotalScore,
	}).Debug("release rejected: total score too low")
}

func (s *NZBService) logReleaseAccepted(media *domain.Media, result *domain.SearchResult, parsed *ParsedNZB, validationScore, qualityScore, totalScore int) {
	log.WithFields(log.Fields{
		"traktID":         media.TraktID,
		"mediaTitle":      media.Title,
		"nzbTitle":        result.Title,
		"resolution":      parsed.Resolution,
		"source":          parsed.Source,
		"codec":           parsed.Codec,
		"validationScore": validationScore,
		"qualityScore":    qualityScore,
		"totalScore":      totalScore,
	}).Info("release accepted")
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
	}).Debug("found season packs, using season pack instead of single episode")
}

func (s *NZBService) logSeasonPacksPassedBlacklist(media *domain.Media, total, passed int) {
	log.WithFields(log.Fields{
		"title":       media.Title,
		"totalFound":  total,
		"passed":      passed,
		"blacklisted": total - passed,
	}).Debug("season packs passed blacklist filter, using season packs")
}

func (s *NZBService) logAllSeasonPacksBlacklisted(media *domain.Media, total int) {
	log.WithFields(log.Fields{
		"title":       media.Title,
		"totalFound":  total,
		"blacklisted": total,
	}).Debug("all season packs blacklisted, falling back to single episode search")
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
	}).Debug("nzb validation and insertion complete")
}

func (s *NZBService) logBestNZBSelected(traktID int64, best *domain.NZB, allNZBs []domain.NZB) {
	fields := log.Fields{
		"traktID":         traktID,
		"candidateCount":  len(allNZBs),
		"title":           best.Title,
		"resolution":      best.Resolution,
		"source":          best.Source,
		"codec":           best.Codec,
		"validationScore": best.ValidationScore,
		"qualityScore":    best.QualityScore,
		"totalScore":      best.TotalScore,
	}

	if len(allNZBs) > minNZBListForComparison {
		topNZBs := findTopNZBs(allNZBs, topNZBCountForLogging)
		for i, nzb := range topNZBs {
			if i == 0 {
				continue // Skip the best (already logged above)
			}
			prefix := fmt.Sprintf("runner%d", i)
			fields[prefix+"Title"] = nzb.Title
			fields[prefix+"TotalScore"] = nzb.TotalScore
			fields[prefix+"Resolution"] = nzb.Resolution
			fields[prefix+"Source"] = nzb.Source
		}
	}

	log.WithFields(fields).Debug("selected best scored nzb")
}
