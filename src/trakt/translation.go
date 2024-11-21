package trakt

type TranslationListParams struct {
	BasicListParams

	Language string `json:"-" url:"-"`
}

type Translation struct {
	Title    string `json:"title"`
	Overview string `json:"overview"`
	Tagline  string `json:"tagline"`
	Language string `json:"language"`
}

type TranslationIterator struct{ BasicIterator }

func (t *TranslationIterator) Translation() (*Translation, error) {
	rcv := &Translation{}
	return rcv, t.Scan(rcv)
}
