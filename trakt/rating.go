package trakt

import "time"

type ListRatingParams struct {
	ListParams

	Type    Type    `json:"-" url:"-"`
	Ratings []int64 `json:"-" url:"-"`
}

type RatingDistribution struct {
	Score        float64          `json:"rating"`
	Votes        int64            `json:"votes"`
	Distribution map[string]int64 `json:"distribution"`
}

type Rating struct {
	GenericMediaElement
	Season  *Season   `json:"season"`
	Score   float64   `json:"rating"`
	RatedAt time.Time `json:"rated_at"`
}

type RatingIterator struct{ Iterator }

func (r *RatingIterator) Rating() (*Rating, error) {
	rcv := &Rating{}
	return rcv, r.Scan(rcv)
}
