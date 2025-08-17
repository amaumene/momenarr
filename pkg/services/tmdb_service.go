package services

import (
	"github.com/cyruzin/golang-tmdb"
	"github.com/sirupsen/logrus"
)

type TMDBService struct {
	client *tmdb.Client
	apiKey string
}

func NewTMDBService(apiKey string) *TMDBService {
	tmdbClient, err := tmdb.Init(apiKey)
	if err != nil {
		logrus.WithError(err).Error("Failed to initialize TMDB client")
		return nil
	}

	return &TMDBService{
		client: tmdbClient,
		apiKey: apiKey,
	}
}

func (t *TMDBService) GetFrenchTitle(mediaType string, tmdbID int64, fallbackTitle string) string {
	if tmdbID == 0 || t.client == nil {
		return fallbackTitle
	}

	frenchTitle := t.fetchFrenchTitle(mediaType, tmdbID, fallbackTitle)

	if frenchTitle == "" {
		frenchTitle = fallbackTitle
	}

	t.logFrenchTitle(tmdbID, mediaType, fallbackTitle, frenchTitle)
	return frenchTitle
}

func (t *TMDBService) fetchFrenchTitle(mediaType string, tmdbID int64, fallbackTitle string) string {
	options := map[string]string{"language": "fr-FR"}

	if mediaType == "episode" || mediaType == "show" || mediaType == "series" {
		return t.fetchTVFrenchTitle(tmdbID, options, fallbackTitle)
	}
	return t.fetchMovieFrenchTitle(tmdbID, options, fallbackTitle)
}

func (t *TMDBService) fetchTVFrenchTitle(tmdbID int64, options map[string]string, fallbackTitle string) string {
	tvDetails, err := t.client.GetTVDetails(int(tmdbID), options)
	if err != nil {
		logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB TV show details")
		return fallbackTitle
	}
	return tvDetails.Name
}

func (t *TMDBService) fetchMovieFrenchTitle(tmdbID int64, options map[string]string, fallbackTitle string) string {
	movieDetails, err := t.client.GetMovieDetails(int(tmdbID), options)
	if err != nil {
		logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB movie details")
		return fallbackTitle
	}
	return movieDetails.Title
}

func (t *TMDBService) logFrenchTitle(tmdbID int64, mediaType, fallbackTitle, frenchTitle string) {
	logrus.WithFields(logrus.Fields{
		"tmdb_id":    tmdbID,
		"media_type": mediaType,
		"english":    fallbackTitle,
		"french":     frenchTitle,
	}).Debug("Retrieved French title from TMDB")
}

func (t *TMDBService) GetOriginalLanguage(mediaType string, tmdbID int64) string {
	if tmdbID == 0 || t.client == nil {
		return ""
	}

	originalLanguage := t.fetchOriginalLanguage(mediaType, tmdbID)
	t.logOriginalLanguage(tmdbID, mediaType, originalLanguage)
	return originalLanguage
}

func (t *TMDBService) fetchOriginalLanguage(mediaType string, tmdbID int64) string {
	if mediaType == "episode" || mediaType == "show" || mediaType == "series" {
		return t.fetchTVOriginalLanguage(tmdbID)
	}
	return t.fetchMovieOriginalLanguage(tmdbID)
}

func (t *TMDBService) fetchTVOriginalLanguage(tmdbID int64) string {
	tvDetails, err := t.client.GetTVDetails(int(tmdbID), nil)
	if err != nil {
		logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB TV show details for language")
		return ""
	}
	return tvDetails.OriginalLanguage
}

func (t *TMDBService) fetchMovieOriginalLanguage(tmdbID int64) string {
	movieDetails, err := t.client.GetMovieDetails(int(tmdbID), nil)
	if err != nil {
		logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB movie details for language")
		return ""
	}
	return movieDetails.OriginalLanguage
}

func (t *TMDBService) logOriginalLanguage(tmdbID int64, mediaType, originalLanguage string) {
	logrus.WithFields(logrus.Fields{
		"tmdb_id":           tmdbID,
		"media_type":        mediaType,
		"original_language": originalLanguage,
	}).Debug("Retrieved original language from TMDB")
}
