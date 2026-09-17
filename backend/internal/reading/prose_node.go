package reading

// ReadingTarget belongs to the same publication as its containing document.
// Qualify each actionable target, so a delayed renderer event cannot silently
// acquire the revision of a newer reading session. Empty Anchor means start.
type ReadingTarget struct {
	ChapterIndex    int    `json:"chapterIndex"`
	ContentRevision int64  `json:"contentRevision"`
	Anchor          string `json:"anchor,omitempty"`
}

// Block is a semantic prose node, not an HTML element or provider-native value.
// Version 1 uses only paragraph.Text and image.Resource/Alt. Version 2 represents
// prose recursively: paragraphs/containers have Children, text leaves have Text.
// Optional fields belong only to the corresponding finite node kinds; the EPUB
// reading projection constructs these explicitly rather than copying its tree.
type Block struct {
	Kind        string             `json:"kind"`
	Text        string             `json:"text,omitempty"`
	Resource    *ResourceReference `json:"resource,omitempty"`
	Alt         string             `json:"alt,omitempty"`
	ID          string             `json:"id,omitempty"`
	Language    string             `json:"language,omitempty"`
	Direction   string             `json:"direction,omitempty"`
	Role        string             `json:"role,omitempty"`
	Children    []Block            `json:"children,omitempty"`
	Level       int                `json:"level,omitempty"`
	Start       *int               `json:"start,omitempty"`
	ColSpan     int                `json:"colSpan,omitempty"`
	RowSpan     int                `json:"rowSpan,omitempty"`
	Target      *ReadingTarget     `json:"target,omitempty"`
	URL         string             `json:"url,omitempty"`
	Unavailable bool               `json:"unavailable,omitempty"`
	Width       int                `json:"width,omitempty"`
	Height      int                `json:"height,omitempty"`
}
