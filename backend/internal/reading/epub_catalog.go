package reading

import "github.com/otwako/novelreader/internal/epub"

// epubCatalog keeps the section inventory and contents hierarchy separate.
// Like epubContent, this proof is not registered with a live provider yet.
func epubCatalog(revision int64, prepared epub.Preparation) Catalog {
	chapters := make([]Chapter, len(prepared.Sections))
	for index, section := range prepared.Sections {
		chapters[index] = Chapter{Index: index, Title: section.Title, Auxiliary: !section.Main}
	}
	source := "publication"
	if prepared.Navigation.Source == "spine" {
		source = "sections"
	}
	return Catalog{Chapters: chapters, ContentRevision: revision, Navigation: &Navigation{Source: source, Entries: epubNavigationEntries(revision, prepared.Navigation.Entries)}}
}

func epubNavigationEntries(revision int64, entries []epub.ResolvedNavigationEntry) []NavigationEntry {
	out := make([]NavigationEntry, len(entries))
	for i, entry := range entries {
		node := NavigationEntry{Label: entry.Label, Unavailable: entry.Unavailable, Children: epubNavigationEntries(revision, entry.Children)}
		if entry.Target != nil && !entry.Unavailable {
			node.Target = &ReadingTarget{ChapterIndex: entry.Target.Section, ContentRevision: revision, Anchor: entry.Target.Anchor}
		}
		out[i] = node
	}
	return out
}
