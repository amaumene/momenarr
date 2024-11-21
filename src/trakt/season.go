package trakt

import "time"

type Season struct {
	MediaIDs `json:"ids"`

	Title        string    `json:"title"`
	Overview     string    `json:"overview"`
	Number       int64     `json:"number"`
	Rating       float64   `json:"rating"`
	Votes        int64     `json:"votes"`
	EpisodeCount int64     `json:"episode_count"`
	AiredCount   int64     `json:"aired_episodes"`
	FirstAired   time.Time `json:"first_aired"`
	Network      string    `json:"network"`
}

type SeasonIterator struct{ Iterator }

func (s *SeasonIterator) Season() (*Season, error) {
	rcv := &Season{}
	return rcv, s.Scan(rcv)
}

type SeasonWithEpisodes struct {
	Season
	Episodes []*Episode `json:"episodes"`
}

type SeasonWithEpisodesIterator struct{ BasicIterator }

func (s *SeasonWithEpisodesIterator) Season() (*SeasonWithEpisodes, error) {
	rcv := &SeasonWithEpisodes{}
	return rcv, s.Scan(rcv)
}
