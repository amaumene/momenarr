package trakt

import "time"

type ListPlaybackParams struct {
	Params

	Type  Type  `json:"-" url:"-"`
	Limit int64 `json:"-" url:"limit,omitempty"`
}

type RemovePlaybackParams struct {
	Params

	Type Type `json:"-" url:"-"`
}

type basePlaybackItem struct {
	GenericMediaElement
	ID      int64          `json:"id"`
	Sharing *SharingParams `json:"sharing"`
}

type Playback struct {
	basePlaybackItem
	Progress float64   `json:"progress"`
	PausedAt time.Time `json:"paused_at"`
}

type PlaybackIterator struct{ BasicIterator }

func (p *PlaybackIterator) Playback() (*Playback, error) {
	rcv := &Playback{}
	return rcv, p.Scan(rcv)
}
