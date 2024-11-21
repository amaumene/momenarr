package trakt

// Certification represents a movie or TV Show age certification.
type Certification struct {
	// Name the name of the certification.
	Name string `json:"name"`
	// Slug the URL compatible version of the certification
	Slug Slug `json:"slug"`
	// Description a description for the certification, can be empty.
	Description string `json:"description"`
}

// CertificationIterator represents a list of certification entries which can be iterated.
type CertificationIterator struct{ BasicIterator }

// Certification attempts to return an Certification entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (c *CertificationIterator) Certification() (*Certification, error) {
	rcv := &Certification{}
	return rcv, c.Scan(rcv)
}
