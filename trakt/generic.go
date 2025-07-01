package trakt

import (
	"errors"
	"reflect"
	"strconv"
	"sync"
	"time"
)

type Status string

const (
	StatusReturningSeries Status = `returning series`
	StatusInProduction    Status = `in production`
	StatusPlanned         Status = `planned`
	StatusCancelled       Status = `canceled`
	StatusEnded           Status = `ended`
	StatusPostProduction  Status = `post production`
	StatusRumored         Status = `rumored`
	StatusReleased        Status = `released`
)

// ExtendedParams params which can be used when a request supports
// asking for additional information.
type ExtendedParams struct {
	BasicParams

	Extended ExtendedType `url:"extended" json:"-"`
}

// ExtendedParams params which can be used when a request supports
// asking for additional information.
type ExtendedListParams struct {
	BasicListParams

	Extended ExtendedType `url:"extended" json:"-"`
}

// FilterListParams params which can be used when a listing option
// accepts filters which can be applied.
type FilterListParams struct {
	BasicListParams
	Filters

	Extended ExtendedType `url:"extended" json:"-"`
}

type SearchID interface {
	id() string
	path() string
}

type ID int64

func (i ID) id() string   { return strconv.Itoa(int(i)) }
func (i ID) path() string { return "trakt" }

type Slug string

func (s Slug) id() string   { return string(s) }
func (s Slug) path() string { return "trakt" }

type IMDB string

func (i IMDB) id() string   { return string(i) }
func (i IMDB) path() string { return "imdb" }

type TVDB int64

func (t TVDB) id() string   { return strconv.Itoa(int(t)) }
func (t TVDB) path() string { return "tvdb" }

type TMDB int64

func (t TMDB) id() string   { return strconv.Itoa(int(t)) }
func (t TMDB) path() string { return "tmdb" }

type baseIDs struct {
	Slug Slug `json:"slug"`
}

type objectIds struct {
	baseIDs
	Trakt ID `json:"trakt"`
}

type MediaIDs struct {
	Slug   Slug `json:"slug,omitempty" url:"-"`
	Trakt  ID   `json:"trakt,omitempty" url:"-"`
	TVDB   TVDB `json:"tvdb,omitempty" url:"-"`
	IMDB   IMDB `json:"imdb,omitempty" url:"-"`
	TMDB   TMDB `json:"tmdb,omitempty" url:"-"`
	TVRage int  `json:"tvrage,omitempty" url:"-"`
}

type Type string

func (t Type) String() string { return string(t) }

// Plural returns the plural for a type.
func (t Type) Plural() string {
	if t == TypeAll {
		return t.String()
	}
	return t.String() + "s"
}

const (
	TypeMovie   Type = `movie`
	TypeShow    Type = `show`
	TypeSeason  Type = `season`
	TypeEpisode Type = `episode`
	TypeList    Type = `list`
	TypePerson  Type = `person`
	TypeAll          = Type(All)
)

type SharingParams struct {
	Twitter bool `json:"twitter"`
	Tumblr  bool `json:"tumblr"`
	Medium  bool `json:"medium"`
}

type GenericElementParams struct {
	IDs MediaIDs `json:"ids,omitempty" url:"-"`

	Title string `json:"title,omitempty" url:"-"`
	Year  int64  `json:"year,omitempty" url:"-"`
}

type commonElements struct {
	MediaIDs `json:"ids"`

	Title                 string    `json:"title"`
	Overview              string    `json:"overview"`
	Rating                float64   `json:"rating"`
	Votes                 int64     `json:"votes"`
	AvailableTranslations []string  `json:"available_translations"`
	Runtime               int64     `json:"runtime"`
	UpdatedAt             time.Time `json:"updated_at"`
	Comments              int64     `json:"comment_count"`
}

type topLevelMediaElement struct {
	Type Type `json:"type"`

	Show  *Show  `json:"show"`
	Movie *Movie `json:"movie"`
}

type GenericMediaElement struct {
	topLevelMediaElement
	Episode *Episode `json:"episode"`
}

type GenericMediaElementIterator struct {
	Iterator

	mu         sync.RWMutex
	currentPtr *GenericMediaElement
}

// current helper function to capture the current pointer onto a value
// stored on the iterator itself.
// this is so we can have more helpful functions attached to the iterator
// to get specific types.
// we need to perform locking on this function as we are potentially concurrently
// reading and writing to `currentPtr`.
// typical iterators are already concurrent-safe, so this is only required as
// we are performing additional functionality.
func (li *GenericMediaElementIterator) current() (*GenericMediaElement, error) {
	var ptr *GenericMediaElement
	li.mu.Lock()
	ptr = li.currentPtr
	li.mu.Unlock()

	if ptr != nil {
		return ptr, nil
	}

	err := li.Scan(ptr)
	if err != nil {
		return nil, err
	}

	li.mu.Lock()
	defer li.mu.Unlock()
	li.currentPtr = ptr

	return ptr, nil
}

func (li *GenericMediaElementIterator) Type() (Type, error) {
	cur, err := li.current()
	if err != nil {
		return "", err
	}

	return cur.Type, nil
}

func (li *GenericMediaElementIterator) Show() (*Show, error) {
	cur, err := li.current()
	if err != nil {
		return nil, err
	}

	return cur.Show, nil
}

func (li *GenericMediaElementIterator) Movie() (*Movie, error) {
	cur, err := li.current()
	if err != nil {
		return nil, err
	}

	return cur.Movie, nil
}

type GenericElement struct {
	GenericMediaElement

	List *List `json:"list"`
}

type ListByTypeParams struct {
	BasicParams

	Type Type `json:"type"`
}

// IDPath helper function to format a search id into a search URL.
func IDPath(id SearchID) string { return "/search/" + id.path() + "/%s" }

// parseYear helper function to parse a year from a string or float.
func parseYear(i interface{}) (int64, error) {
	v := reflect.ValueOf(i)

	// as per the standard json docs
	// if the type if not defined (interface{})
	// the unmarshaller will default to float64
	// for numbers.
	// so we will either have a choice of float64 or string.
	switch v.Kind() {
	case reflect.Float64:
		return int64(v.Float()), nil
	case reflect.String:
		s := v.String()
		if s != "" {
			return strconv.ParseInt(s, 10, 64)
		}
	}

	return 0, errors.New("invalid type")
}
