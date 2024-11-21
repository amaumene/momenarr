package trakt

import (
	"encoding/json"
	"time"
)

type Action string

const (
	ActionWatch    Action = "watch"
	ActionCheckin  Action = "checkin"
	ActionScrobble Action = "scrobble"
)

type MediaType string

const (
	MediaTypeDigital   MediaType = "digital"
	MediaTypeBluray    MediaType = "bluray"
	MediaTypeHDDVD     MediaType = "hddvd"
	MediaTypeVCD       MediaType = "vcd"
	MediaTypeVHS       MediaType = "vhs"
	MediaTypeDVD       MediaType = "dvd"
	MediaTypeBetaMax   MediaType = "betamax"
	MediaTypeLaserDisc MediaType = "laserdisc"
)

type Resolution string

const (
	ResolutionUHD     Resolution = "uhd_4k"
	ResolutionHD1080p Resolution = "hd_1080p"
	ResolutionHD1080i Resolution = "hd_1080i"
	ResolutionHD720p  Resolution = "hd_720p"
	ResolutionSD480p  Resolution = "sd_480p"
	ResolutionSD480i  Resolution = "sd_480i"
	ResolutionSD576p  Resolution = "sd_576p"
	ResolutionSD576i  Resolution = "sd_576i"
)

type HDR string

const (
	HDRDolbyVision HDR = "dolby_vision"
	HDR10          HDR = "hdr10"
	HDR10Plus      HDR = "hdr10_plus"
	HDRHLG         HDR = "hlg"
)

type Audio string

const (
	AudioLPCM             Audio = "lpcm"
	AudioMP3              Audio = "mp3"
	AudioAAC              Audio = "acc"
	AudioOGG              Audio = "ogg"
	AudioWMA              Audio = "wma"
	AudioDTS              Audio = "dts"
	AudioDTSMA            Audio = "dts_ma"
	AudioDTSHR            Audio = "dts_hr"
	AudioDTSX             Audio = "dts_x"
	AudioAuro3D           Audio = "auro_3d"
	AudioDolbyDigital     Audio = "dolby_digital"
	AudioDolbyDigitalPlus Audio = "dolby_digital_plus"
	AudioDolbyAtmos       Audio = "dolby_atmos"
	AudioDolbyTrueHD      Audio = "dolby_truehd"
	AudioDolbyPrologic    Audio = "dolby_prologic"
)

type ListCollectionParams struct {
	ListParams

	Type     Type         `json:"-" url:"-"`
	Extended ExtendedType `url:"extended" json:"-"`
}

type ListWatchedParams = ListCollectionParams

type ListHistoryParams struct {
	ListParams

	Type Type     `json:"-" url:"-"`
	ID   SearchID `json:"-" url:"-"`

	StartAt  time.Time    `url:"start_at" json:"-"`
	EndAt    time.Time    `url:"end_at" json:"-"`
	Extended ExtendedType `url:"extended" json:"-"`
}

// Metadata to assign to the collection object.
type Metadata struct {
	Type          MediaType  `json:"media_type,omitempty"`
	Resolution    Resolution `json:"resolution,omitempty"`
	HDR           HDR        `json:"hdr,omitempty"`
	Audio         Audio      `json:"audio,omitempty"`
	AudioChannels string     `json:"audio_channels,omitempty"`
	ThreeD        bool       `json:"3d,omitempty"`
}

type MediaCollectionParams struct {
	Metadata

	IDs         MediaIDs  `json:"ids,omitempty" url:"-"`
	Title       string    `json:"title,omitempty" url:"-"`
	Year        int64     `json:"year,omitempty" url:"-"`
	CollectedAt time.Time `json:"collected_at,omitempty" url:"-"`
}

type EpisodeCollectionParams struct {
	Metadata

	Number      int64     `json:"number"`
	CollectedAt time.Time `json:"collected_at,omitempty" url:"-"`
}

type SeasonCollectionParams struct {
	Metadata

	Number      int64                      `json:"number"`
	CollectedAt time.Time                  `json:"collected_at,omitempty" url:"-"`
	Episodes    []*EpisodeCollectionParams `json:"episodes,omitempty"`
}

type ShowCollectionParams struct {
	Metadata

	IDs         MediaIDs                  `json:"ids,omitempty" url:"-"`
	Title       string                    `json:"title,omitempty" url:"-"`
	Year        int64                     `json:"year,omitempty" url:"-"`
	CollectedAt time.Time                 `json:"collected_at,omitempty" url:"-"`
	Seasons     []*SeasonCollectionParams `json:"seasons,omitempty" url:"-"`
}

type AddToCollectionParams struct {
	Params

	Movies   []*MediaCollectionParams `json:"movies,omitempty" url:"-"`
	Seasons  []*MediaCollectionParams `json:"seasons,omitempty" url:"-"`
	Episodes []*MediaCollectionParams `json:"episodes,omitempty" url:"-"`
	Shows    []*ShowCollectionParams  `json:"shows,omitempty" url:"-"`
}

type EpisodeHistoryParams struct {
	Number    int64     `json:"number"`
	WatchedAt time.Time `json:"watched_at,omitempty" url:"-"`
}

type SeasonHistoryParams struct {
	Number    int64                   `json:"number"`
	WatchedAt time.Time               `json:"watched_at,omitempty" url:"-"`
	Episodes  []*EpisodeHistoryParams `json:"episodes,omitempty"`
}

type ShowHistoryParams struct {
	IDs       MediaIDs               `json:"ids,omitempty" url:"-"`
	Title     string                 `json:"title,omitempty" url:"-"`
	Year      int64                  `json:"year,omitempty" url:"-"`
	WatchedAt time.Time              `json:"watched_at,omitempty" url:"-"`
	Seasons   []*SeasonHistoryParams `json:"seasons,omitempty" url:"-"`
}

type MediaHistoryParams struct {
	IDs       MediaIDs  `json:"ids,omitempty" url:"-"`
	Title     string    `json:"title,omitempty" url:"-"`
	Year      int64     `json:"year,omitempty" url:"-"`
	WatchedAt time.Time `json:"watched_at,omitempty" url:"-"`
}

type AddToHistoryParams struct {
	Params

	Movies   []*MediaHistoryParams `json:"movies,omitempty" url:"-"`
	Seasons  []*MediaHistoryParams `json:"seasons,omitempty" url:"-"`
	Episodes []*MediaHistoryParams `json:"episodes,omitempty" url:"-"`
	Shows    []*ShowHistoryParams  `json:"shows,omitempty" url:"-"`
}

type EpisodeRatingParams struct {
	Number  int64     `json:"number"`
	Rating  int64     `json:"rating,omitempty"`
	RatedAt time.Time `json:"rated_at,omitempty" url:"-"`
}

type SeasonRatingParams struct {
	Number   int64                  `json:"number"`
	Rating   int64                  `json:"rating,omitempty"`
	RatedAt  time.Time              `json:"rated_at,omitempty" url:"-"`
	Episodes []*EpisodeRatingParams `json:"episodes,omitempty"`
}

type ShowRatingParams struct {
	IDs     MediaIDs              `json:"ids,omitempty" url:"-"`
	Title   string                `json:"title,omitempty" url:"-"`
	Year    int64                 `json:"year,omitempty" url:"-"`
	Rating  int64                 `json:"rating,omitempty"`
	RatedAt time.Time             `json:"rated_at,omitempty" url:"-"`
	Seasons []*SeasonRatingParams `json:"seasons,omitempty" url:"-"`
}

type MediaRatingParams struct {
	IDs     MediaIDs  `json:"ids,omitempty" url:"-"`
	Title   string    `json:"title,omitempty" url:"-"`
	Year    int64     `json:"year,omitempty" url:"-"`
	Rating  int64     `json:"rating,omitempty"`
	RatedAt time.Time `json:"rated_at,omitempty" url:"-"`
}

type AddRatingsParams struct {
	Params

	Movies   []*MediaRatingParams `json:"movies,omitempty" url:"-"`
	Seasons  []*MediaRatingParams `json:"seasons,omitempty" url:"-"`
	Episodes []*MediaRatingParams `json:"episodes,omitempty" url:"-"`
	Shows    []*ShowRatingParams  `json:"shows,omitempty" url:"-"`
}

type MediaWatchListParams struct {
	IDs   MediaIDs `json:"ids,omitempty" url:"-"`
	Title string   `json:"title,omitempty" url:"-"`
	Year  int64    `json:"year,omitempty" url:"-"`
}

type SeasonWatchListParams struct {
	Number      int64     `json:"number"`
	CollectedAt time.Time `json:"collected_at,omitempty" url:"-"`
	Episodes    []int64   `json:"episodes,omitempty"`
}

func (s *SeasonWatchListParams) MarshalJSON() ([]byte, error) {
	type A SeasonWatchListParams
	type B struct {
		Number int64 `json:"number"`
	}
	type C struct {
		A
		Episodes []*B `json:"episodes,omitempty"`
	}

	var c = &C{A: A{Number: s.Number}, Episodes: make([]*B, len(s.Episodes))}
	for idx, epNo := range s.Episodes {
		c.Episodes[idx] = &B{epNo}
	}

	return json.Marshal(c)
}

type ShowWatchListParams struct {
	IDs     MediaIDs                  `json:"ids,omitempty" url:"-"`
	Title   string                    `json:"title,omitempty" url:"-"`
	Year    int64                     `json:"year,omitempty" url:"-"`
	Seasons []*SeasonCollectionParams `json:"seasons,omitempty" url:"-"`
}

type AddToWatchListParams struct {
	Params

	Movies   []*MediaWatchListParams `json:"movies,omitempty" url:"-"`
	Seasons  []*MediaWatchListParams `json:"seasons,omitempty" url:"-"`
	Episodes []*MediaWatchListParams `json:"episodes,omitempty" url:"-"`
	Shows    []*ShowWatchListParams  `json:"shows,omitempty" url:"-"`
}

type MediaRemovalParams struct {
	IDs   MediaIDs `json:"ids,omitempty" url:"-"`
	Title string   `json:"title,omitempty" url:"-"`
	Year  int64    `json:"year,omitempty" url:"-"`
}

type SeasonRemovalParams struct {
	Number   int64   `json:"number" url:"-"`
	Episodes []int64 `json:"-" url:"-"`
}

func (s *SeasonRemovalParams) MarshalJSON() ([]byte, error) {
	type A SeasonRemovalParams
	type B struct {
		Number int64 `json:"number"`
	}
	type C struct {
		A
		Episodes []*B `json:"episodes,omitempty"`
	}

	var c = &C{A: A{Number: s.Number}, Episodes: make([]*B, len(s.Episodes))}
	for idx, epNo := range s.Episodes {
		c.Episodes[idx] = &B{epNo}
	}

	return json.Marshal(c)
}

type ShowRemovalParams struct {
	IDs     MediaIDs               `json:"ids,omitempty" url:"-"`
	Title   string                 `json:"title,omitempty" url:"-"`
	Year    int64                  `json:"year,omitempty" url:"-"`
	Seasons []*SeasonRemovalParams `json:"seasons,omitempty"`
}

type RemoveFromCollectionParams struct {
	Params

	Movies   []*MediaRemovalParams `json:"movies,omitempty" url:"-"`
	Seasons  []*MediaRemovalParams `json:"seasons,omitempty" url:"-"`
	Episodes []*MediaRemovalParams `json:"episodes,omitempty" url:"-"`
	Shows    []*ShowRemovalParams  `json:"shows,omitempty" url:"-"`
}

type RemoveFromHistoryParams struct {
	Params

	Movies   []*MediaRemovalParams `json:"movies,omitempty" url:"-"`
	Seasons  []*MediaRemovalParams `json:"seasons,omitempty" url:"-"`
	Episodes []*MediaRemovalParams `json:"episodes,omitempty" url:"-"`
	Shows    []*ShowRemovalParams  `json:"shows,omitempty" url:"-"`
	IDs      []int64               `json:"ids,omitempty" url:"-"`
}

type RemoveFromWatchListParams = RemoveFromCollectionParams
type RemoveRatingsParams = RemoveFromCollectionParams

type ListWatchListParams struct {
	ListParams

	Type     Type         `url:"-" json:"-"`
	Sort     SortType     `url:"-" json:"-"`
	Extended ExtendedType `url:"extended" json:"-"`
}

type commonMediaActivity struct {
	LastRated       time.Time `json:"rated_at"`
	LastWatchListed time.Time `json:"watchlisted_at"`
	LastCommented   time.Time `json:"commented_at"`
}

type ShowActivity struct {
	commonMediaActivity
	LastHidden time.Time `json:"hidden_at"`
}

type SeasonActivity = ShowActivity

type EpisodeActivity struct {
	commonMediaActivity
	LastWatched   time.Time `json:"watched_at"`
	LastCollected time.Time `json:"collected_at"`
	LastPaused    time.Time `json:"paused_at"`
}

type MovieActivity struct {
	EpisodeActivity
	LastHidden time.Time `json:"hidden_at"`
}

type CommentActivity struct {
	LastLiked time.Time `json:"liked_at"`
}

type ListActivity struct {
	CommentActivity
	LastUpdated   time.Time `json:"updated_at"`
	LastCommented time.Time `json:"commented_at"`
}

type AccountActivity struct {
	LastUpdatedSettings time.Time `json:"settings_at"`
}

type LastActivity struct {
	LastUpdated time.Time        `json:"all"`
	Account     *AccountActivity `json:"account"`
	Lists       *ListActivity    `json:"lists"`
	Comments    *CommentActivity `json:"comments"`
	Seasons     *SeasonActivity  `json:"seasons"`
	Shows       *ShowActivity    `json:"shows"`
	Episodes    *EpisodeActivity `json:"episodes"`
	Movies      *MovieActivity   `json:"movies"`
}

type CollectionIterator interface {
	BasicIterator

	Type() Type
	Show() (*CollectedShow, error)
	Movie() (*CollectedMovie, error)
}

type CollectedMovie struct {
	Movie `json:"movie"`

	CollectedAt time.Time `json:"collected_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Metadata    *Metadata `json:"metadata"`
}

type CollectedShow struct {
	Show `json:"show"`

	LastCollectedAt time.Time          `json:"last_collected_at"`
	LastUpdatedAt   time.Time          `json:"last_updated_at"`
	Seasons         []*CollectedSeason `json:"seasons"`
}

type numberedEntity struct {
	Number int64 `json:"number"`
}

type CollectedSeason struct {
	numberedEntity
	Episodes []*CollectedEpisode
}

type CollectedEpisode struct {
	numberedEntity
	CollectedAt time.Time `json:"collected_at"`
	Metadata    *Metadata `json:"metadata"`
}

type watchedDetails struct {
	LastWatchedAt time.Time `json:"last_watched_at"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
	Plays         int64     `json:"plays"`
}

type WatchedIterator interface {
	BasicIterator

	Type() Type
	Show() (*WatchedShow, error)
	Movie() (*WatchedMovie, error)
}

type WatchedMovie struct {
	watchedDetails
	Movie `json:"movie"`
}

type WatchedShow struct {
	watchedDetails
	Show    `json:"show"`
	ResetAt time.Time        `json:"reset_at"`
	Seasons []*WatchedSeason `json:"seasons"`
}

type WatchedSeason struct {
	numberedEntity
	Episodes []*WatchedEpisode `json:"episodes"`
}

type WatchedEpisode struct {
	numberedEntity
	Plays         int64     `json:"plays"`
	LastWatchedAt time.Time `json:"last_watched_at"`
}

type NotFound struct {
	Movies   []*GenericElementParams `json:"movies"`
	Shows    []*GenericElementParams `json:"shows"`
	Seasons  []*GenericElementParams `json:"seasons"`
	Episodes []*GenericElementParams `json:"episodes"`
}

type ChangeSet struct {
	Movies   int64 `json:"movies"`
	Episodes int64 `json:"episodes"`
}

type AddToCollectionResult struct {
	Added    *ChangeSet `json:"added"`
	Updated  *ChangeSet `json:"updated"`
	Existing *ChangeSet `json:"existing"`
	NotFound *NotFound  `json:"not_found"`
}

type AddToHistoryResult = AddToCollectionResult
type AddRatingsResult = AddToCollectionResult
type AddToWatchListResult = AddToCollectionResult

type RemoveFromCollectionResult struct {
	Deleted  *ChangeSet `json:"deleted"`
	NotFound *NotFound  `json:"not_found"`
}

type RemoveFromWatchListResult = RemoveFromCollectionResult
type RemoveRatingsResult = RemoveFromCollectionResult

type History struct {
	GenericMediaElement

	ID        int64     `json:"id"`
	Action    Action    `json:"action"`
	WatchedAt time.Time `json:"watched_at"`
}

type HistoryIterator struct{ Iterator }

func (h *HistoryIterator) History() (*History, error) {
	rcv := &History{}
	return rcv, h.Scan(rcv)
}

type RemoveFromHistoryResult struct {
	RemoveFromCollectionResult
	NotFound *struct {
		NotFound
		IDs []int64 `json:"ids"`
	} `json:"not_found"`
}

type WatchListEntry struct {
	GenericMediaElement
	Season   *Season   `json:"season"`
	Rank     int64     `json:"rank"`
	ListedAt time.Time `json:"listed_at"`
}

type SortPreference struct {
	Type      SortType
	Direction SortDirection
}

type WatchListEntryIterator struct{ Iterator }

func (w *WatchListEntryIterator) Entry() (*WatchListEntry, error) {
	rcv := &WatchListEntry{}
	return rcv, w.Scan(rcv)
}

// Applied attempts to retrieve the applied sort type on
// a users watchlist.
func (w *WatchListEntryIterator) Applied() *SortPreference {
	h := w.headers()
	if h == nil {
		return nil
	}

	return &SortPreference{
		Type:      SortType(h.Get("X-Applied-Sort-By")),
		Direction: SortDirection(h.Get("X-Applied-Sort-How")),
	}
}

// Preferred attempts to retrieve the preferred sort type on
// a users watchlist.
func (w *WatchListEntryIterator) Preferred() *SortPreference {
	h := w.headers()
	if h == nil {
		return nil
	}

	return &SortPreference{
		Type:      SortType(h.Get("X-Sort-By")),
		Direction: SortDirection(h.Get("X-Sort-How")),
	}
}
