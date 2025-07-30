package services

import (
	"strconv"

	"github.com/cyruzin/golang-tmdb"
	"github.com/sirupsen/logrus"
)

type TMDBService struct {
	client *tmdb.Client
}

func NewTMDBService(apiKey string) *TMDBService {
	tmdbClient, err := tmdb.Init(apiKey)
	if err != nil {
		logrus.WithError(err).Error("Failed to initialize TMDB client")
		return nil
	}

	return &TMDBService{
		client: tmdbClient,
	}
}

func (t *TMDBService) GetFrenchTitle(mediaType string, tmdbID int64, fallbackTitle string) string {
	if tmdbID == 0 || t.client == nil {
		return fallbackTitle
	}

	var frenchTitle string

	if mediaType == "episode" || mediaType == "show" || mediaType == "series" {
		// Get TV show details with French language
		options := map[string]string{
			"language": "fr-FR",
		}
		tvDetails, err := t.client.GetTVDetails(int(tmdbID), options)
		if err != nil {
			logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB TV show details")
			return fallbackTitle
		}
		frenchTitle = tvDetails.Name
	} else {
		// Get movie details with French language
		options := map[string]string{
			"language": "fr-FR",
		}
		movieDetails, err := t.client.GetMovieDetails(int(tmdbID), options)
		if err != nil {
			logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB movie details")
			return fallbackTitle
		}
		frenchTitle = movieDetails.Title
	}

	// Use fallback if no French title
	if frenchTitle == "" {
		frenchTitle = fallbackTitle
	}

	logrus.WithFields(logrus.Fields{
		"tmdb_id":      tmdbID,
		"media_type":   mediaType,
		"english":      fallbackTitle,
		"french":       frenchTitle,
	}).Debug("Retrieved French title from TMDB")

	return frenchTitle
}

func (t *TMDBService) GetOriginalLanguage(mediaType string, tmdbID int64) string {
	if tmdbID == 0 || t.client == nil {
		return ""
	}

	var originalLanguage string

	if mediaType == "episode" || mediaType == "show" || mediaType == "series" {
		// Get TV show details (no language parameter to get original data)
		tvDetails, err := t.client.GetTVDetails(int(tmdbID), nil)
		if err != nil {
			logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB TV show details for language")
			return ""
		}
		originalLanguage = tvDetails.OriginalLanguage
	} else {
		// Get movie details (no language parameter to get original data)
		movieDetails, err := t.client.GetMovieDetails(int(tmdbID), nil)
		if err != nil {
			logrus.WithError(err).WithField("tmdb_id", tmdbID).Warn("Failed to fetch TMDB movie details for language")
			return ""
		}
		originalLanguage = movieDetails.OriginalLanguage
	}

	logrus.WithFields(logrus.Fields{
		"tmdb_id":           tmdbID,
		"media_type":        mediaType,
		"original_language": originalLanguage,
	}).Debug("Retrieved original language from TMDB")

	return originalLanguage
}

// GetMovieByTitle searches for a movie by title and returns TMDB ID and original language
func (t *TMDBService) GetMovieByTitle(title string, year int) (int64, string, error) {
	if t.client == nil {
		return 0, "", nil
	}

	options := map[string]string{}
	if year > 0 {
		options["year"] = strconv.Itoa(year)
	}

	searchResults, err := t.client.GetSearchMovies(title, options)
	if err != nil {
		return 0, "", err
	}

	if len(searchResults.Results) == 0 {
		return 0, "", nil
	}

	// Return the first result
	movie := searchResults.Results[0]
	return int64(movie.ID), movie.OriginalLanguage, nil
}

// GetTVShowByTitle searches for a TV show by title and returns TMDB ID and original language
func (t *TMDBService) GetTVShowByTitle(title string, year int) (int64, string, error) {
	if t.client == nil {
		return 0, "", nil
	}

	options := map[string]string{}
	if year > 0 {
		options["first_air_date_year"] = strconv.Itoa(year)
	}

	searchResults, err := t.client.GetSearchTVShow(title, options)
	if err != nil {
		return 0, "", err
	}

	if len(searchResults.Results) == 0 {
		return 0, "", nil
	}

	// Return the first result
	show := searchResults.Results[0]
	return int64(show.ID), show.OriginalLanguage, nil
}