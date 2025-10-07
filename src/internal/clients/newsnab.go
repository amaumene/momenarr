package clients

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/newsnab"
)

const (
	parseIntBase    = 10
	parseIntBitSize = 64
)

type newsnabAdapter struct {
	host   string
	apiKey string
}

func NewNewsnabAdapter(host, apiKey string) domain.NZBSearcher {
	return &newsnabAdapter{
		host:   host,
		apiKey: apiKey,
	}
}

func (a *newsnabAdapter) SearchMovie(ctx context.Context, imdb string) ([]domain.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	xmlResponse, err := newsnab.SearchMovie(imdb, a.host, a.apiKey)
	if err != nil {
		return nil, fmt.Errorf("searching movie: %w", err)
	}

	return a.parseResults(xmlResponse)
}

func (a *newsnabAdapter) SearchEpisode(ctx context.Context, imdb string, season, episode int64) ([]domain.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	xmlResponse, err := newsnab.SearchTVShow(imdb, season, episode, a.host, a.apiKey)
	if err != nil {
		return nil, fmt.Errorf("searching episode: %w", err)
	}

	return a.parseResults(xmlResponse)
}

func (a *newsnabAdapter) SearchSeasonPack(ctx context.Context, imdb string, season int64) ([]domain.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	xmlResponse, err := newsnab.SearchTVShowSeason(imdb, season, a.host, a.apiKey)
	if err != nil {
		return nil, fmt.Errorf("searching season pack: %w", err)
	}

	return a.parseResults(xmlResponse)
}

func (a *newsnabAdapter) parseResults(xmlResponse string) ([]domain.SearchResult, error) {
	var feed newsnab.Feed
	if err := xml.Unmarshal([]byte(xmlResponse), &feed); err != nil {
		return nil, fmt.Errorf("unmarshalling xml: %w", err)
	}

	return convertFromNewsnabItems(feed.Channel.Items)
}

func convertFromNewsnabItems(items []newsnab.Item) ([]domain.SearchResult, error) {
	results := make([]domain.SearchResult, 0, len(items))
	for _, item := range items {
		length, err := strconv.ParseInt(item.Enclosure.Length, parseIntBase, parseIntBitSize)
		if err != nil {
			continue
		}

		results = append(results, domain.SearchResult{
			Title:  item.Title,
			Link:   item.Enclosure.URL,
			Length: length,
		})
	}
	return results, nil
}
