package trakt

import (
	"encoding/json"
	"strings"
	"time"
)

// StartCheckinParams parameters in order to start a checkin
// operation. The only required values are Type, Element and an
// OAuth token.
type StartCheckinParams struct {
	Params

	// Type the type of media element. Can either be TypeMovie or TypeEpisode.
	Type Type `json:"-"`
	// Element the actual element data. We can provide as much or as little as
	// we want about the item. The recommended values are either the trakt ID or slug.
	Element *GenericElementParams `json:"-"`

	// Message used for sharing. If not sent, it will use the watching string in the user settings.
	Message string `json:"message,omitempty"`

	// Foursquare venue ID. Optional.
	VenueID string `json:"venue_id,omitempty"`
	// Foursquare venue name. Optional.
	VenueName string `json:"venue_name,omitempty"`

	// Version number of the app. Optional.
	AppVersion string `json:"app_version,omitempty"`
	// Build date of the app. Optional.
	AppDate string `json:"app_date,omitempty"`

	// The sharing object is optional and will apply the user's settings if not sent.
	// If sharing is sent, each key will override the user's setting for that social network.
	// Send true to post or false to not post on the indicated social network. You can see which
	// social networks a user has connected with the /users/settings method.
	Sharing *SharingParams `json:"sharing,omitempty"`
}

// MarshalJSON implements Marshaller interface.
func (s *StartCheckinParams) MarshalJSON() ([]byte, error) {
	m := marshalToMap(s)
	m[strings.ToLower(string(s.Type))] = s.Element
	return json.Marshal(m)
}

// Checkin represents a manual checkin entry.
type Checkin struct {
	basePlaybackItem

	// WatchedAt when the user started watching this item.
	WatchedAt time.Time `json:"watched_at"`
}

// UnmarshalJSON implements Unmarshaller interface.
// allows us to determine the type of entry it is
// from the data retrieved.
func (c *Checkin) UnmarshalJSON(bytes []byte) error {
	type A Checkin
	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	switch {
	case a.Episode != nil:
		a.Type = TypeEpisode
	case a.Movie != nil:
		a.Type = TypeMovie
	}

	*c = Checkin(*a)
	return nil
}
