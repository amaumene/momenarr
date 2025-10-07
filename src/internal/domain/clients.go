package domain

import "context"

type DownloadInput struct {
	Filename   string
	Content    string
	Category   string
	DupeMode   string
	Parameters map[string]string
}

type QueueItem struct {
	NZBID   int64
	NZBName string
}

type HistoryItem struct {
	NZBID int64
}

type DownloadClient interface {
	Append(ctx context.Context, input *DownloadInput) (int64, error)
	ListGroups(ctx context.Context) ([]QueueItem, error)
	History(ctx context.Context, includeHidden bool) ([]HistoryItem, error)
	DeleteFromHistory(ctx context.Context, downloadID int64) error
}

type SearchResult struct {
	Title  string
	Link   string
	Length int64
}

type NZBSearcher interface {
	SearchMovie(ctx context.Context, imdb string) ([]SearchResult, error)
	SearchEpisode(ctx context.Context, imdb string, season, episode int64) ([]SearchResult, error)
	SearchSeasonPack(ctx context.Context, imdb string, season int64) ([]SearchResult, error)
}
