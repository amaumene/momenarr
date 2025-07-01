package trakt

import (
	"net/url"

	"github.com/google/go-querystring/query"
)

type SearchField string

func (s SearchField) String() string { return string(s) }

const (
	// Shared search fields for media elements (movie|episode|show)
	SearchFieldTitle       SearchField = "title"
	SearchFieldOverview    SearchField = "overview"
	SearchFieldPeople      SearchField = "people"
	SearchFieldTranslation SearchField = "translations"
	SearchFieldAlias       SearchField = "aliases"

	// search fields specifically for movies.
	SearchFieldTagline SearchField = "tagline"

	// Shared search fields for both person and list.
	SearchFieldName SearchField = "name"

	// search fields specifically for person
	SearchFieldBiography SearchField = "biography"

	// search fields specifically for list.
	SearchFieldDescription SearchField = "description"
)

// TextQueryFilters represents the set of filters which can be applied
// when performing a Text Lookup. this differs from the generic filter
// set as we want to force queryFunc to be given.
type TextQueryFilters struct {
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

type SearchQueryParams struct {
	BasicListParams

	Filters  TextQueryFilters `json:"-" url:"-"`
	Type     Type             `json:"-" url:"-"`
	Query    string           `json:"-" url:"query"`
	Fields   []SearchField    `json:"-" url:"fields,comma,omitempty"`
	Extended ExtendedType     `json:"-" url:"extended,omitempty"`
}

// EncodeValues implements the query.Encoder interface.
func (s *SearchQueryParams) EncodeValues(_ string, v *url.Values) error {
	type A SearchQueryParams
	fv, err := query.Values(s.Filters)
	if err != nil {
		return err
	}

	pv, err := query.Values(A(*s))
	if err != nil {
		return err
	}

	for key, set := range pv {
		(*v)[key] = set
	}

	for key, set := range fv {
		(*v)[key] = set
	}

	return nil
}

type IDLookupParams struct {
	BasicListParams

	Type     []Type       `json:"-" url:"type,comma,omitempty"`
	Extended ExtendedType `json:"-" url:"extended,omitempty"`
}

// SearchResult represents a result from performing a search either by
// test search or ID lookup.
type SearchResult struct {
	GenericElement

	Person *Person `json:"person"`
}

// SearchResultIterator an instance of an iterator which allows us to
// return the current pointer as a concrete SearchResult struct.
type SearchResultIterator struct{ Iterator }

func (s *SearchResultIterator) Result() (*SearchResult, error) {
	rcv := &SearchResult{}
	return rcv, s.Scan(rcv)
}
