package trakt

import (
	"encoding/json"
	"time"
)

type Airs struct {
	Day      string
	Time     string
	Timezone string
}

type Show struct {
	commonElements `json:",inline"`

	Year          int64     `json:"-"`
	FirstAired    time.Time `json:"first_aired"`
	Airs          *Airs     `json:"airs"`
	Certification string    `json:"certification"`
	Country       string    `json:"country"`
	TrailerURL    string    `json:"trailer"`
	HomepageURL   string    `json:"homepage"`
	Status        Status    `json:"status"`
	Genres        []string  `json:"genre"`
	AiredEpisodes int64     `json:"aired_episodes"`
	Language      string    `json:"language"`
}

func (s *Show) UnmarshalJSON(bytes []byte) error {
	type B Show
	type A struct {
		B
		Year interface{} `json:"year"`
	}

	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return nil
	}

	a.B.Year, err = parseYear(a.Year)
	if err != nil {
		return err
	}

	*s = Show(a.B)
	return nil
}

type ShowIterator struct{ Iterator }

func (s *ShowIterator) Show() (*Show, error) {
	rcv := &Show{}
	return rcv, s.Scan(rcv)
}

type TrendingShow struct {
	Show     `json:"show"`
	Watchers int64 `json:"watchers"`
}

type TrendingShowIterator struct{ Iterator }

func (t *TrendingShowIterator) Trending() (*TrendingShow, error) {
	rcv := &TrendingShow{}
	return rcv, t.Scan(rcv)
}

type RecentlyUpdatedShow struct {
	Show      `json:"show"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecentlyUpdatedShowIterator struct{ Iterator }

func (r *RecentlyUpdatedShowIterator) Show() (*RecentlyUpdatedShow, error) {
	rcv := &RecentlyUpdatedShow{}
	return rcv, r.Scan(rcv)
}

type ShowWithStatistics struct {
	statistics
	Show `json:"show"`
}

type ShowWithStatisticsIterator struct{ Iterator }

func (s *ShowWithStatisticsIterator) Show() (*ShowWithStatistics, error) {
	rcv := &ShowWithStatistics{}
	return rcv, s.Scan(rcv)
}

type AnticipatedShow struct {
	Show      `json:"show"`
	ListCount int64 `json:"list_count"`
}

type AnticipatedShowIterator struct{ Iterator }

func (a *AnticipatedShowIterator) Show() (*AnticipatedShow, error) {
	rcv := &AnticipatedShow{}
	return rcv, a.Scan(rcv)
}
