package trakt

// Alias represents an alias for a show or movie
// this is where the name is different based on the language.
type Alias struct {
	// Name the aliased name of the show or movie.
	Name string `json:"name"`
	// Code the language code.
	Code string `json:"code"`
}

// AliasIterator represents a list of alias entries which can be iterated.
type AliasIterator struct{ BasicIterator }

// Alias attempts to return an Alias entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (a *AliasIterator) Alias() (*Alias, error) {
	rcv := &Alias{}
	return rcv, a.Scan(rcv)
}
