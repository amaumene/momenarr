package trakt

import "time"

type ActivityType string

const (
	ActivityTypeAired   ActivityType = "aired"
	ActivityTypeWatched ActivityType = "watched"
)

type ProgressParams struct {
	Params

	Hidden        bool         `json:"-" url:"hidden,omitempty"`
	Specials      bool         `json:"-" url:"specials,omitempty"`
	CountSpecials bool         `json:"-" url:"count_specials,omitempty"`
	LastActivity  ActivityType `json:"-" url:"last_activity,omitempty"`
}

type progress struct {
	Aired   int64 `json:"aired"`
	Watched int64 `json:"completed"`
	// HiddenSeasons *Season
	NextEpisode *Episode `json:"next_episode"`
	LastEpisode *Episode `json:"last_episode"`
}

type CollectedProgress struct {
	progress
	Seasons     []*CollectedSeasonProgress `json:"seasons"`
	CollectedAt time.Time                  `json:"last_collected_at"`
}

type WatchedProgress struct {
	progress
	Seasons   []*WatchedSeasonProgress `json:"seasons"`
	WatchedAt time.Time                `json:"last_watched_at"`
	ResetAt   time.Time                `json:"reset_at"`
}

type seasonProgress struct {
	Number  int64 `json:"number"`
	Aired   int64 `json:"aired"`
	Watched int64 `json:"completed"`
}

type WatchedSeasonProgress struct {
	seasonProgress
	Episodes []*WatchedEpisodeProgress `json:"episodes"`
}

type CollectedSeasonProgress struct {
	seasonProgress
	Episodes []*CollectedEpisodeProgress `json:"episodes"`
}

type episodeProgress struct {
	Number  int64 `json:"number"`
	Watched bool  `json:"completed"`
}

type WatchedEpisodeProgress struct {
	episodeProgress
	WatchedAt time.Time `json:"watched_at"`
}

type CollectedEpisodeProgress struct {
	episodeProgress
	CollectedAt time.Time `json:"collected_at"`
}
