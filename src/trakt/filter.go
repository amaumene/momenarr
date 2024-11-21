package trakt

import (
	"fmt"
	"net/url"
	"strconv"
)

// All the all constant, this is shared between different types
// so cant be allocated to a specific filter.
const All string = `all`

type ExtendedType string

func (e ExtendedType) String() string { return string(e) }

const (
	ExtendedTypeGuestStars         ExtendedType = `guest_stars`
	ExtendedTypeEpisodes           ExtendedType = `episodes`
	ExtendedTypeCollectionMetadata ExtendedType = `metadata`
	ExtendedTypeNoSeasons          ExtendedType = `noseasons`
	ExtendedTypeVip                ExtendedType = `vip`
	ExtendedTypeFull               ExtendedType = `full`
)

type TimePeriod string

func (t TimePeriod) String() string { return string(t) }

const (
	TimePeriodWeekly  TimePeriod = "weekly"
	TimePeriodMonthly TimePeriod = "monthly"
	TimePeriodYearly  TimePeriod = "yearly"
	TimePeriodAll                = TimePeriod(All)
)

type SortType string

func (s SortType) String() string { return string(s) }

const (
	// these sort types are used for comment sorting.
	SortTypeNewest  SortType = "newest"
	SortTypeOldest  SortType = "oldest"
	SortTypeReplies SortType = "replies"
	SortTypeHighest SortType = "highest"
	SortTypeLowest  SortType = "lowest"
	SortTypePlays   SortType = "plays"

	// these sort types are used for list sorting.
	SortTypePopular  SortType = "popular"
	SortTypeLikes    SortType = "likes"
	SortTypeComments SortType = "comments"
	SortTypeItems    SortType = "items"
	SortTypeAdded    SortType = "added"
	SortTypeUpdated  SortType = "updated"

	// these sort types are used for watchlist sorting.
	SortTypeRank       SortType = "rank"
	SortTypeTitle      SortType = "title"
	SortTypeReleased   SortType = "released"
	SortTypeRuntime    SortType = "runtime"
	SortTypePopularity SortType = "popularity"
	SortTypePercentage SortType = "percentage"
	SortTypeVotes      SortType = "votes"
)

type SortDirection string

const (
	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"
)

// Range represents a range type filter.
type Range struct {
	To   int64
	From int64
}

// format formats the range into the correct form which is:
// "<from>-<to>".
func (r *Range) format() string {
	if r.From == 0 && r.To == 0 {
		return ""
	}

	if r.To == 0 {
		return strconv.Itoa(int(r.From))
	}

	return fmt.Sprintf(`%d-%d`, r.From, r.To)
}

// EncodeValues implements Encoder interface
// takes a range and encodes it as a string in the form "<from>-<to>"
func (r *Range) EncodeValues(key string, v *url.Values) error {
	s := r.format()
	if s == "" {
		return nil
	}

	v.Set(key, s)
	return nil
}

// Filters represents the set of filters which can be applied to certain API
// endpoints.
type Filters struct {
	// Common filters.
	Query     string   `url:"query,omitempty" json:"-"`
	Years     []int64  `url:"years,comma,omitempty" json:"-"`
	Genres    []string `url:"genres,comma,omitempty" json:"-"`
	Languages []string `url:"languages,comma,omitempty" json:"-"`
	Countries []string `url:"countries,comma,omitempty" json:"-"`
	Runtime   *Range   `url:"runtimes,omitempty" json:"-"`
	Rating    *Range   `url:"ratings,comma,omitempty" json:"-"`

	// filters specific for movies and shows
	Certifications []string `url:"certifications,comma,omitempty" json:"-"`

	// filters specific for shows
	Networks []string `url:"networks,comma,omitempty" json:"-"`
	Statuses []Status `url:"statuses,comma,omitempty" json:"-"`
}
