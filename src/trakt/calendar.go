package trakt

import (
	"encoding/json"
	"time"
)

// CalendarParams the parameters required to perform
// queries against a calendar where an OAuth token is required.
type CalendarParams struct {
	// ListParams is the parameters which all requests can take for listing
	// based operations.
	ListParams
	// Filters a optional set of filters to apply to a query.
	Filters

	// StartDate the time to start from.
	StartDate time.Time `url:"-"`
	// Days the number of days from the StartDate to query up to.
	Days int64 `url:"-"`
	// Extended the level of detail to return.
	Extended ExtendedType `url:"extended"`
}

// CalendarParams the parameters required to perform
// queries against a calendar where no authorization is required.
type BasicCalendarParams struct {
	// BasicListParams is the parameters which all requests can take for listing
	// based operations where no OAuth token is required.
	BasicListParams
	// Filters a optional set of filters to apply to a query.
	Filters

	// StartDate the time to start from.
	StartDate time.Time `url:"-"`
	// Days the number of days from the StartDate to query up to.
	Days int64 `url:"-"`
	// Extended the level of detail to return.
	Extended ExtendedType `url:"extended"`
}

// CalendarShow represents a show with additional
// information relating to a calendar entry.
type CalendarShow struct {
	// Show the show information.
	Show `json:"show"`

	// Episode the episode which is airing.
	Episode *Episode `json:"episode"`
	// FirstAired when this episode was first aired.
	FirstAired time.Time `json:"first_aired"`
}

// CalendarMovie represents a movie with additional
// information relating to a calendar entry.
type CalendarMovie struct {
	// Movie the movie information.
	Movie `json:"movie"`
	// Released when this movie was released.
	Released time.Time `json:"-"`
}

// UnmarshalJSON implements the Unmarshaller interface.
// allows us to parse the released date into a time.
func (c *CalendarMovie) UnmarshalJSON(bytes []byte) error {
	type B CalendarMovie
	type A struct {
		B
		Released string `json:"released"`
	}

	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	a.B.Released, err = time.Parse(`2006-01-02`, a.Released)
	*c = CalendarMovie(a.B)
	return err
}

// CalendarShowIterator represents a list of calendar shows which can be iterated.
type CalendarShowIterator struct{ Iterator }

// Entry attempts to return an CalendarShow entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (li *CalendarShowIterator) Entry() (*CalendarShow, error) {
	rcv := &CalendarShow{}
	return rcv, li.Scan(rcv)
}

// CalendarMovieIterator represents a list of calendar movies which can be iterated.
type CalendarMovieIterator struct{ Iterator }

// Entry attempts to return an CalendarMovie entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (li *CommentIterator) Entry() (*CalendarMovie, error) {
	rcv := &CalendarMovie{}
	return rcv, li.Scan(rcv)
}
