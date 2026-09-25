// Package reading coordinates provider-owned content with shared library state.
package reading

type Chapter struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	IsVolume bool   `json:"isVolume"`
	// Auxiliary sections are readable/bookmarkable, but never main progress.
	Auxiliary bool `json:"auxiliary,omitempty"`
}

type Catalog struct {
	Chapters        []Chapter   `json:"chapters"`
	ContentRevision int64       `json:"contentRevision"`
	Syncing         bool        `json:"-"`
	Navigation      *Navigation `json:"navigation,omitempty"`
}

const (
	DocumentVersion           = 1
	StructuredDocumentVersion = 2
)

type ResourceReference struct {
	Unavailable bool   `json:"unavailable,omitempty"`
	Href        string `json:"href"`
	MediaType   string `json:"mediaType,omitempty"`
}
type Document struct {
	CoverPlaceholder bool    `json:"coverPlaceholder,omitempty"`
	Kind             string  `json:"kind"`
	Title            string  `json:"title"`
	Blocks           []Block `json:"blocks"`
}
type Content struct {
	FreshForMS      *int64   `json:"freshForMs,omitempty"`
	ContentRevision int64    `json:"contentRevision"`
	Version         int      `json:"version"`
	Document        Document `json:"document"`
	OfflineCopy     bool     `json:"offlineCopy,omitempty"`
}

func prose(revision int64, title string, blocks []Block) Content {
	return Content{ContentRevision: revision, Version: DocumentVersion, Document: Document{Kind: "prose", Title: title, Blocks: blocks}}
}
