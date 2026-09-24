package reading

// Navigation is a publication's contents hierarchy, not its main reading order.
// Source distinguishes authored navigation from an explicitly labeled section list.
type Navigation struct {
	Source  string            `json:"source"` // "publication" or "sections"
	Entries []NavigationEntry `json:"entries"`
}

type NavigationEntry struct {
	Label       string            `json:"label"`
	Target      *ReadingTarget    `json:"target,omitempty"`
	Unavailable bool              `json:"unavailable,omitempty"`
	Children    []NavigationEntry `json:"children,omitempty"`
}
