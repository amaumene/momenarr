package trakt

import "time"

type Privacy string

func (p Privacy) String() string { return string(p) }

const (
	PrivacyPrivate Privacy = "private"
	PrivacyPublic  Privacy = "public"
)

type ListType string

func (l ListType) String() string { return string(l) }

const (
	ListTypeAll                = ListType(All)
	ListTypePersonal  ListType = "personal"
	ListTypeOfficial  ListType = "official"
	ListTypeWatchlist ListType = "watchlists"
)

type GetListParams struct {
	BasicListParams

	SortType SortType `json:"-" url:"-"`
	ListType ListType `json:"-" url:"-"`
}

type List struct {
	objectIds `json:"ids"`

	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Privacy        Privacy   `json:"privacy"`
	DisplayNumbers bool      `json:"display_numbers"`
	AllowComments  bool      `json:"allow_comments"`
	SortBy         string    `json:"sort_by"`
	SortDirection  string    `json:"sort_how"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Items          int64     `json:"item_count"`
	Comments       int64     `json:"comment_count"`
	User           *User     `json:"user"`
}

type ListIterator struct{ Iterator }

func (l *ListIterator) List() (*List, error) {
	rcv := &List{}
	return rcv, l.Scan(rcv)
}

// RecentList represents a list with the most
// recent like and comment figures, usually over the last
// 7 days. The like and comment counts on the list are for
// all time.
type RecentList struct {
	List     `json:"list"`
	Likes    int64 `json:"like_count"`
	Comments int64 `json:"comment_count"`
}

type RecentListIterator struct{ Iterator }

func (r *RecentListIterator) RecentList() (*RecentList, error) {
	rcv := &RecentList{}
	return rcv, r.Scan(rcv)
}
