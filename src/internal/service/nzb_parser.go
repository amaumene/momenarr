package service

import (
	"strings"

	tnp "github.com/ProfChaos/torrent-name-parser"
	"github.com/amaumene/momenarr/internal/domain"
)

type ParsedNZB struct {
	Title      string
	Year       int64
	Season     int64
	Episode    int64
	Resolution string
	Source     string
	Codec      string
	Proper     bool
	Repack     bool
}

func parseSearchResult(result *domain.SearchResult) (*ParsedNZB, error) {
	torrent, err := tnp.ParseName(result.Title)
	if err != nil {
		return nil, err
	}

	return &ParsedNZB{
		Title:      extractTitle(&torrent),
		Year:       extractYear(&torrent),
		Season:     extractSeason(&torrent),
		Episode:    extractEpisode(&torrent),
		Resolution: extractResolution(&torrent),
		Source:     extractSource(&torrent),
		Codec:      extractCodec(&torrent),
		Proper:     torrent.Proper,
		Repack:     torrent.Repack,
	}, nil
}

func extractTitle(torrent *tnp.Torrent) string {
	return normalizeTitle(string(torrent.Title))
}

func normalizeTitle(title string) string {
	normalized := strings.ToLower(title)
	normalized = strings.TrimSpace(normalized)
	return normalized
}

func extractYear(torrent *tnp.Torrent) int64 {
	return int64(torrent.Year)
}

func extractSeason(torrent *tnp.Torrent) int64 {
	return int64(torrent.Season)
}

func extractEpisode(torrent *tnp.Torrent) int64 {
	return int64(torrent.Episode)
}

func extractResolution(torrent *tnp.Torrent) string {
	return strings.ToUpper(string(torrent.Resolution))
}

func extractSource(torrent *tnp.Torrent) string {
	return strings.ToUpper(string(torrent.Source))
}

func extractCodec(torrent *tnp.Torrent) string {
	return strings.ToUpper(string(torrent.Codec))
}
