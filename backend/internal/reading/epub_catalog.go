package reading

import "github.com/otwako/novelreader/internal/epub"

// epubCatalog projects the prepared section inventory, not the hierarchical TOC.
// Like epubContent, this proof is not registered with a live provider yet.
func epubCatalog(revision int64, prepared epub.Preparation) Catalog {
	chapters := make([]Chapter, len(prepared.Sections))
	for index, section := range prepared.Sections {
		chapters[index] = Chapter{Index: index, Title: section.Title, Auxiliary: !section.Main}
	}
	return Catalog{Chapters: chapters, ContentRevision: revision}
}
