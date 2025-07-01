package trakt

import "time"

type EpisodeListParams struct {
	BasicParams

	TranslationLanguage string `url:"translations,omitempty" json:"-"`
}

type Episode struct {
	commonElements `json:",inline"`

	Season     int64     `json:"season"`
	Number     int64     `json:"number"`
	Absolute   int64     `json:"number_abs"`
	FirstAired time.Time `json:"first_aired"`
}

type EpisodeIterator struct{ Iterator }

func (e *EpisodeIterator) Episode() (*Episode, error) {
	rcv := &Episode{}
	return rcv, e.Scan(rcv)
}

type EpisodeWithTranslations struct {
	Episode
	Translations []*Translation `json:"translations"`
}

type EpisodeWithTranslationsIterator struct{ BasicIterator }

func (e *EpisodeWithTranslationsIterator) Episode() (*EpisodeWithTranslations, error) {
	rcv := &EpisodeWithTranslations{}
	return rcv, e.Scan(rcv)
}
