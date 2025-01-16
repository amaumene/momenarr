package newsnab

import "encoding/xml"

type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	Language    string   `xml:"language"`
	WebMaster   string   `xml:"webMaster"`
	Response    Response `xml:"response"`
	Items       []Item   `xml:"item"`
}

type Response struct {
	Offset string `xml:"offset,attr"`
	Total  string `xml:"total,attr"`
}

type Item struct {
	Title       string    `xml:"title"`
	GUID        GUID      `xml:"guid"`
	Link        string    `xml:"link"`
	Comments    string    `xml:"comments"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate"`
	Enclosure   Enclosure `xml:"enclosure"`
	NewznabAttr []Attr    `xml:"attr,omitempty"` // Captures newznab-specific attributes
}

type GUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type Attr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}
