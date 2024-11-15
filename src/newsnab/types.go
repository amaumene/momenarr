package newsnab

type Feed struct {
	Attributes FeedAttributes `json:"@attributes"`
	Channel    Channel        `json:"channel"`
}

type FeedAttributes struct {
	Version string `json:"version"`
}

type Channel struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Link        string   `json:"link"`
	Language    string   `json:"language"`
	WebMaster   string   `json:"webMaster"`
	Category    struct{} `json:"category"`
	Response    Response `json:"response"`
	Item        []Item   `json:"item"`
}

type Response struct {
	Attributes ResponseAttributes `json:"@attributes"`
}

type ResponseAttributes struct {
	Offset string `json:"offset"`
	Total  string `json:"total"`
}

type Item struct {
	Title       string    `json:"title"`
	GUID        string    `json:"guid"`
	Link        string    `json:"link"`
	Comments    string    `json:"comments"`
	PubDate     string    `json:"pubDate"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Enclosure   Enclosure `json:"enclosure"`
}

type Enclosure struct {
	Attributes EnclosureAttributes `json:"@attributes"`
}

type EnclosureAttributes struct {
	URL    string `json:"url"`
	Length string `json:"length"`
	Type   string `json:"type"`
}
