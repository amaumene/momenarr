package trakt

type statistics struct {
	Watchers   int64 `json:"watcher_count"`
	Plays      int64 `json:"play_count"`
	Collected  int64 `json:"collected_count"`
	Collectors int64 `json:"collector_count"`
}

type Statistics struct {
	Watchers  int64 `json:"watchers"`
	Plays     int64 `json:"plays"`
	Collected int64 `json:"collectors"`
	Comments  int64 `json:"comments"`
	Lists     int64 `json:"lists"`
	Votes     int64 `json:"votes"`
}
