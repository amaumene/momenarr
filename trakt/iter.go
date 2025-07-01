package trakt

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
)

// NoLimit defines that we have no limit on the amount
// of pages to paginate through.
const NoLimit int64 = -1

// default pagination values.
const (
	defaultPage  = 1
	defaultLimit = 10
)

// listMeta is the structure that contains the common properties
// of List iterators. The Count property is only populated if the
// total_count include option is passed in (see tests for example).
type listMeta struct {
	currentPage int64
	limit       int64
	totalPages  int64
	totalCount  int64
}

func (l *listMeta) meta() *listMeta { return l }

// unmarshalHeaders allows us to unmarshal a response from
// the response HTTP headers.
// this is the case for pagination values where they are supplied
// in headers and not the response.
func (f *frame) unmarshalHeaders(h http.Header) error {
	(*f).listMeta = &listMeta{
		limit:       parseInt(h.Get(`X-Pagination-limit`)),
		currentPage: parseInt(h.Get(`X-Pagination-Page`)),
		totalPages:  parseInt(h.Get(`X-Pagination-Page-Count`)),
		totalCount:  parseInt(h.Get(`X-Pagination-Item-Count`)),
	}

	(*f).h = h
	return nil
}

// newEmptyFrame generates a new empty frame which is
// available to consume the next "frame" of results.
func newEmptyFrame() iterationFrame {
	rcv := make([]*json.RawMessage, 0)
	return &frame{r: &rcv, listMeta: &listMeta{}}
}

// frame a frame is a wrapped set of results for a single
// "frame" basically a single page in the entire result.
type frame struct {
	// the pagination information, derived from headers.
	*listMeta
	// the raw response for the current frame.
	// we utilise json.RawMessage so that the initial
	// json.Unmarshal on this frame receiver just
	// confirms that we can unmarshal into a slice.
	// we hold each entry in the slice as the raw JSON.
	// this is done for two reasons:
	// - we dont have to know what structure to unmarshal into.
	// - its more performant to unmarshal each entry as needed
	//   rather than unmarshal the entire set and not use them all.
	r *[]*json.RawMessage
	// the set of headers from the response, this is updated
	// on every frame, however, we only store the first set on the iterator.
	// used to retrieve variables that may be passed in headers
	// which dont change during the lifetime of the iterator
	// i.e we retrieve sorting headers when getting a users watchlist, these
	// dont change between frames.
	h http.Header
}

// rcv implements iterationFrame, returns the pointer to
// the interface which the raw HTTP body should be unmarshalled into.
func (f *frame) rcv() *[]*json.RawMessage { return f.r }

// headers returns the headers from the HTTP request.
func (f *frame) headers() http.Header { return f.h }

// iterationFrame represents a window or slice of
// an entire pagination result.
type iterationFrame interface {
	// rcv returns the receiver which the response data is unmarshalled into.
	rcv() *[]*json.RawMessage
	// meta the current metadata for the position we are in the complete set.
	meta() *listMeta
	// headers returns the headers from the HTTP request.
	headers() http.Header
}

// BasicIterator represents an iterator which does not deal with paging.
// this can be used where the iterator does not require a page limit be set.
type BasicIterator interface {
	// Err retrieves any error that occurred performing
	// an operation to get a "frame". A frame represents a single
	// paginated set of results (the results for a single page)
	Err() error
	// Next moves the cursor to the next point in the results
	// performing an operation to retrieve the next result frame
	// from the API if required.
	Next() bool
	// Scan scans the current data into the supplied receiver.
	Scan(rcv interface{}) error
	// getPage internal function which allows us to retrieve the next page
	// of results.
	getPage() iterationFrame
	// headers retrieves the headers for the initial result.
	headers() http.Header
}

// Iterator a generic representation of an iterator.
type Iterator interface {
	BasicIterator
	// PageLimit sets an absolute limit on how many pages to iterate through.
	// if the result set has less pages than the limit, obviously it will
	// finish before that.
	PageLimit(page int64)
}

// singleIter this type of iterator
// is used when we wat to simulate pagination on
// an entire result.
// this is considered thread-safe and
// all exported functions can be called across
// multiple go-routines.
type singleIter struct {
	sync.RWMutex

	// the current pointer in the result.
	cur *json.RawMessage
	// err is the error from retrieving the current frame.
	err error
	// values is the current set of frame values.
	values []*json.RawMessage
	// query is the function to trigger to retrieve
	// a single frame of results.
	query queryFunc
	// listParams is the initial set of parameters setting the initial
	// page and limit for the frame to use.
	listParams ListParamsContainer
	// lazyLoad controls whether we want to load the initial page upfront
	// or wait until the first call to Next.
	lazyLoad bool
	// loaded whether initial load has been performed.
	loaded bool
	// initialHeaders these are the headers received in the initial response.
	//
	// WARNING on lazy-loaded iterators this will only be available after the
	// initial call to Next.
	initialHeaders http.Header
}

// isLazyLoad returns whether lazy load is enabled on the iterator.
func (s *singleIter) isLazyLoad() bool {
	s.RLock()
	defer s.RUnlock()
	return s.lazyLoad
}

// hasLoaded returns whether the initial load has been performed.
func (s *singleIter) hasLoaded() bool {
	s.RLock()
	defer s.RUnlock()
	return s.loaded
}

// withRLock helper function which wraps a function with a read lock
// enabled.
func (s *singleIter) withRLock(fn func()) {
	s.RLock()
	defer s.RUnlock()
	fn()
}

// withLock helper function which wraps a function with a read/write lock
// enabled.
func (s *singleIter) withLock(fn func()) {
	s.Lock()
	defer s.Unlock()
	fn()
}

// headers returns the headers from the initial response the iterator made.
func (s *singleIter) headers() http.Header {
	s.RLock()
	defer s.RUnlock()
	return s.initialHeaders
}

// Scan attempts to can the current entry in the result set
// into the provided receiver rcv.
// if there is no pointer available or we failed to scan the receiver
// then an error is returned.
func (s *singleIter) Scan(rcv interface{}) error {
	var cur *json.RawMessage
	s.withLock(func() { cur = s.cur })

	if cur == nil {
		return errors.New("nothing left in result set")
	}

	return json.Unmarshal(*cur, rcv)
}

// Err returns the error from performing any query in the lifetime of
// the iterator.
func (s *singleIter) Err() error {
	s.RLock()
	defer s.RUnlock()
	return s.err
}

// PageLimit is not required on a single iterator as it only deals
// with a single page. So the limit is already enforced to 1.
func (s *singleIter) PageLimit(_ int64) {}

// getPage retrieves the next page using the defined query.
func (s *singleIter) getPage() iterationFrame {
	var window iterationFrame
	var cp ListParamsContainer
	s.withRLock(func() {
		cp = s.listParams
	})

	// we pass a copy through so that internally we dont update our reference.
	s.withLock(func() {
		window, s.err = s.query(cp)
		if s.err != nil {
			s.loaded = true
			return
		}

		// if we have a receiver response on the frame.
		if rcv := window.rcv(); rcv != nil {
			s.values = *rcv
		}

		// if this is the initial request, set the headers.
		if !s.loaded {
			s.initialHeaders = window.headers()
		}

		// ensure we always set that we have loaded /
		// attempted to load at least a single frame.
		s.loaded = true
	})

	return window
}

// Next moves the cursor in the result set to the next entry
// and returns true if there is a pointer available.
// this function returns false if no cursors are available in
// the result set.
func (s *singleIter) Next() bool {
	var next bool

	// if we are lazy loading
	// check to see if we have any results.
	if s.isLazyLoad() && !s.hasLoaded() {
		var v []*json.RawMessage
		s.withRLock(func() {
			v = s.values
		})

		if len(v) == 0 {
			s.getPage()
		}
	}

	s.withLock(func() {
		if s.err != nil {
			return
		}

		if len(s.values) == 0 {
			return
		}
		s.cur = s.values[0]
		s.values = s.values[1:]
		next = true
	})

	return next
}

// iter this iterator builds on-top of
// a single iterator and allows us to paginate between
// pages.
// this is considered thread-safe and
// all exported functions can be called across
// multiple go-routines.
type iter struct {
	singleIter
	meta  *listMeta
	limit int64
}

// PageLimit allows us to limit the amount of pages to paginate.
func (it *iter) PageLimit(page int64) {
	it.Lock()
	defer it.Unlock()
	it.limit = page
}

// Next moves the cursor in the result set to the next entry
// and returns true if there is a pointer available.
// this function returns false if no cursors are available in
// the result set.
func (it *iter) Next() bool {
	var (
		err error
		lm  *listMeta
		v   []*json.RawMessage
	)

	it.withLock(func() {
		err, lm, v = it.err, it.meta, it.values
	})

	if err != nil {
		return false
	}

	if it.isLazyLoad() && !it.hasLoaded() {
		// perform the initial page query.
		it.getPage()
		return it.singleIter.Next()
	}

	np := lm.currentPage + 1
	if len(v) == 0 && (lm.totalPages > lm.currentPage) && !it.hasReachedLimit(np) {
		// update the page number.
		it.withLock(func() {
			it.listParams.setPagination(np, lm.limit)
		})

		it.getPage()
	}

	return it.singleIter.Next()
}

// hasReachedLimit determines if we have reached the defined page limit.
func (it *iter) hasReachedLimit(nextPage int64) bool {
	it.Lock()
	defer it.Unlock()
	if it.limit == NoLimit {
		return false
	}

	return it.limit < nextPage
}

// getPage performs the query to retrieve the next frame of data.
func (it *iter) getPage() iterationFrame {
	window := it.singleIter.getPage()
	if window != nil {
		it.withLock(func() {
			it.meta = window.meta()
		})
	}

	return window
}

// queryFunc is the function used to get a page listing.
type queryFunc func(ListParamsContainer) (iterationFrame, error)

// newIterator returns a new iter for a given queryFunc and its options.
//
// lazyLoad determines if the initial page of results is loaded when the iterator is generated
// or when we call our initial call to Next.
func newIterator(p ListParamsContainer, query queryFunc, lazyLoad bool) Iterator {
	iter := &iter{
		singleIter: singleIter{
			listParams: p,
			query:      query,
			lazyLoad:   lazyLoad,
		},
		limit: NoLimit, // by default paginate through all results available.
	}

	// ensure default pagination values are defined.
	p.setDefaultPagination(defaultPage, defaultLimit)

	if !lazyLoad {
		iter.getPage()
	}

	return iter
}

// newSimulatedIterator a simulated iterator is an iterator which
// only works its way though the single frame (result set) it will never
// attempt to move to another page. This is used so we can have all lists of
// results (paginated or not) will utilise an iterator so the library is consistent.
//
// lazyLoad determines if the initial page of results is loaded when the iterator is generated
// or when we call our initial call to Next.
func newSimulatedIterator(p ListParamsContainer, query queryFunc, lazyLoad bool) Iterator {
	iter := &singleIter{
		query:      query,
		listParams: p,
		lazyLoad:   lazyLoad,
	}

	if !lazyLoad {
		iter.getPage()
	}

	return iter
}
