package epub

import "time"

type BookInfo struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	Language    string `json:"language"`
	CoverBase64 string `json:"coverBase64"`
	SpineCount  int    `json:"spineCount"`
}

type TOCEntry struct {
	Title      string     `json:"title"`
	Href       string     `json:"href"`
	SpineIndex int        `json:"spineIndex"`
	Children   []TOCEntry `json:"children,omitempty"`
}

type SpineItem struct {
	IDRef string `json:"idRef"`
	Href  string `json:"href"`
}

type ManifestItem struct {
	ID         string `json:"id"`
	Href       string `json:"href"`
	MediaType  string `json:"mediaType"`
	Properties string `json:"properties,omitempty"`
}

type ReadingPosition struct {
	BookID       string  `json:"bookId"`
	SpineIndex   int     `json:"spineIndex"`
	ScrollOffset float64 `json:"scrollOffset"`
}

type Highlight struct {
	ID         string    `json:"id"`
	BookID     string    `json:"bookId"`
	SpineIndex int       `json:"spineIndex"`
	Text       string    `json:"text"`
	Prefix     string    `json:"prefix,omitempty"`
	Suffix     string    `json:"suffix,omitempty"`
	Color      string    `json:"color"`
	Note       string    `json:"note,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type AppSettings struct {
	Theme      string  `json:"theme"`
	FontSize   float64 `json:"fontSize"`
	FontFamily string  `json:"fontFamily"`
}
