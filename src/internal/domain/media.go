package domain

import "context"

type Media struct {
	TraktID    int64 `boltholdIndex:"Trakt"`
	IMDB       string
	Number     int64
	Season     int64
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	DownloadID int64
}

type NZB struct {
	TraktID int64 `boltholdIndex:"Trakt"`
	Link    string
	Length  int64
	Title   string
	Failed  bool
}

type Notification struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	TraktID  string `json:"trakt"`
	Dir      string `json:"dir"`
}

type MediaRepository interface {
	Insert(ctx context.Context, key int64, media *Media) error
	Update(ctx context.Context, key int64, media *Media) error
	Get(ctx context.Context, key int64) (*Media, error)
	Delete(ctx context.Context, key int64) error
	FindNotOnDisk(ctx context.Context) ([]Media, error)
	FindNotInList(ctx context.Context, traktIDs []int64) ([]Media, error)
	FindWithIMDB(ctx context.Context) ([]Media, error)
	Close() error
}

type NZBRepository interface {
	Insert(ctx context.Context, key string, nzb *NZB) error
	FindByTraktID(ctx context.Context, traktID int64, pattern string, failed bool) ([]NZB, error)
	FindAll(ctx context.Context) ([]NZB, error)
	MarkFailed(ctx context.Context, title string) error
	DeleteByTraktID(ctx context.Context, traktID int64) error
}
