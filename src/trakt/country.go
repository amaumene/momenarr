package trakt

// Country represents a country which includes the
// name and the ISO short code for that country.
type Country struct {
	// Name the name of the country
	Name string `json:"name"`
	// Code the ISO short code for the country.
	Code string `json:"code"`
}

// CountryIterator represents a list of countries which can be iterated.
type CountryIterator struct{ BasicIterator }

// Country attempts to return an Country entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (c *CountryIterator) Country() (*Country, error) {
	rcv := &Country{}
	return rcv, c.Scan(rcv)
}
