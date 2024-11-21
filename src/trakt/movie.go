package trakt

import (
	"encoding/json"
	"time"
)

type ReleaseType string

const (
	ReleaseTypeUnknown    ReleaseType = "unknown"
	ReleaseTypePremiere   ReleaseType = "premiere"
	ReleaseTypeLimited    ReleaseType = "limited"
	ReleaseTypeTheatrical ReleaseType = "theatrical"
	ReleaseTypeDigital    ReleaseType = "digital"
	ReleaseTypePhysical   ReleaseType = "physical"
	ReleaseTypeTV         ReleaseType = "tv"
)

type TimePeriodListParams struct {
	BasicListParams
	Filters

	Period   TimePeriod   `url:"-" json:"-"`
	Extended ExtendedType `url:"extended" json:"-"`
}

type BoxOfficeListParams = ExtendedParams

type RecentlyUpdatedListParams struct {
	BasicListParams

	StartDate time.Time    `json:"-" url:"-"`
	Extended  ExtendedType `url:"extended" json:"-"`
}

type ReleaseListParams struct {
	BasicListParams

	Country string `json:"-" url:"-"`
}

type Movie struct {
	commonElements `json:",inline"`

	Year          int64     `json:"-"`
	Tagline       string    `json:"tagline"`
	Released      time.Time `json:"released"`
	Certification string    `json:"certification"`
	Country       string    `json:"country"`
	TrailerURL    string    `json:"trailer"`
	HomepageURL   string    `json:"homepage"`
	Status        Status    `json:"status"`
	Genres        []string  `json:"genre"`
	Language      string    `json:"language"`
}

func (m *Movie) UnmarshalJSON(bytes []byte) error {
	type B Movie
	type A struct {
		B
		Released string      `json:"released"`
		Year     interface{} `json:"year"`
	}

	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	if len(a.Released) > 0 {
		a.B.Released, err = time.Parse(`2006-01-02`, a.Released)
	}

	a.B.Year, err = parseYear(a.Year)
	if err != nil {
		return err
	}

	*m = Movie(a.B)
	return err
}

type MovieIterator struct{ Iterator }

func (m *MovieIterator) Movie() (*Movie, error) {
	rcv := &Movie{}
	return rcv, m.Scan(rcv)
}

type Release struct {
	Country       string      `json:"country"`
	Certification string      `json:"certification"`
	ReleaseDate   time.Time   `json:"-"`
	ReleaseType   ReleaseType `json:"release_type"`
	Note          string      `json:"note"`
}

func (r *Release) UnmarshalJSON(bytes []byte) error {
	type B Release
	type A struct {
		B
		Released string `json:"release_date"`
	}

	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	if len(a.Released) > 0 {
		a.B.ReleaseDate, err = time.Parse(`2006-01-02`, a.Released)
	}

	*r = Release(a.B)
	return err
}

type ReleaseIterator struct{ BasicIterator }

func (r *ReleaseIterator) Release() (*Release, error) {
	rcv := &Release{}
	return rcv, r.Scan(rcv)
}

type TrendingMovie struct {
	Movie    `json:"movie"`
	Watchers int64 `json:"watchers"`
}

type TrendingMovieIterator struct{ Iterator }

func (t *TrendingMovieIterator) Trending() (*TrendingMovie, error) {
	rcv := &TrendingMovie{}
	return rcv, t.Scan(rcv)
}

type RecentlyUpdatedMovie struct {
	Movie     `json:"movie"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RecentlyUpdatedMovieIterator struct{ Iterator }

func (m *RecentlyUpdatedMovieIterator) Movie() (*RecentlyUpdatedMovie, error) {
	rcv := &RecentlyUpdatedMovie{}
	return rcv, m.Scan(rcv)
}

type MovieWithStatistics struct {
	statistics
	Movie `json:"movie"`
}

type MovieWithStatisticsIterator struct{ Iterator }

func (m *MovieWithStatisticsIterator) Movie() (*MovieWithStatistics, error) {
	rcv := &MovieWithStatistics{}
	return rcv, m.Scan(rcv)
}

type AnticipatedMovie struct {
	ListCount int64 `json:"list_count"`
	Movie     `json:"movie"`
}

type AnticipatedMovieIterator struct{ Iterator }

func (a *AnticipatedMovieIterator) Movie() (*AnticipatedMovie, error) {
	rcv := &AnticipatedMovie{}
	return rcv, a.Scan(rcv)
}

type BoxOfficeMovie struct {
	Revenue int64 `json:"revenue"`
	Movie   `json:"movie"`
}

type BoxOfficeMovieIterator struct{ BasicIterator }

func (m *BoxOfficeMovieIterator) Movie() (*BoxOfficeMovie, error) {
	rcv := &BoxOfficeMovie{}
	return rcv, m.Scan(rcv)
}
