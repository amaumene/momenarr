package trakt

type Genre struct {
	Name string `json:"name"`
	Slug Slug   `json:"slug"`
}

type GenreIterator struct{ BasicIterator }

func (c *GenreIterator) Genre() (*Genre, error) {
	rcv := &Genre{}
	return rcv, c.Scan(rcv)
}
