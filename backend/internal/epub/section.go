package epub

const maxCellSpan = 1000

// Section is preparation evidence, not a wire document. Only Nodes and their
// opaque keys are presentation-safe. Source anchors/references remain private
// until publication-wide target and image validation has resolved the bindings.
// The version-2 wire contract will be frozen with the renderer proof.
type Section struct {
	Cover       bool // explicit body-level EPUB cover semantics, not a filename heuristic
	Title       string
	Root        Node
	Anchors     map[string]string // original ID -> opaque node ID; empty means ambiguous
	Links       map[string]Reference
	Images      map[string]Reference
	Diagnostics []string
}

// Node is a finite semantic tree, never an arbitrary HTML element. Only the
// normalizer and preparation binders populate it; publisher tags/styles are not
// carried through. Text is preserved separately from IDs and link/resource keys.
type Node struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id,omitempty"`
	Text        string         `json:"text,omitempty"`
	Language    string         `json:"language,omitempty"`
	Direction   string         `json:"direction,omitempty"`
	Role        string         `json:"role,omitempty"`
	Children    []Node         `json:"children,omitempty"`
	Level       int            `json:"level,omitempty"`
	Start       *int           `json:"start,omitempty"`
	ColSpan     int            `json:"colSpan,omitempty"`
	RowSpan     int            `json:"rowSpan,omitempty"`
	Target      *SectionTarget `json:"target,omitempty"` // bound only after publication-wide lookup
	Unavailable bool           `json:"unavailable,omitempty"`
	Link        string         `json:"link,omitempty"`  // private binding key for a local target
	URL         string         `json:"url,omitempty"`   // explicit HTTP(S) user action only; never a resource
	Image       string         `json:"image,omitempty"` // private binding key; only validation adds dimensions
	Width       int            `json:"width,omitempty"`
	Height      int            `json:"height,omitempty"`
	Alt         string         `json:"alt,omitempty"`
}
