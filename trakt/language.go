package trakt

type Language struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type LanguageIterator struct{ BasicIterator }

func (c *LanguageIterator) Language() (*Language, error) {
	rcv := &Language{}
	return rcv, c.Scan(rcv)
}
