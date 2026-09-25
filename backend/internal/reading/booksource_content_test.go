package reading

import (
	"fmt"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/processor"
)

func TestBookSourceDocumentNormalizesCacheBlocksAndIssuesResources(t *testing.T) {
	p := BookSource{ImageHref: func(id string, revision int64, chapter, image int, bundleID string) string {
		return fmt.Sprintf("resource:%s:%d:%d:%d", id, revision, chapter, image)
	}}
	entry := book.CachedChapter{BookID: "book", ContentRevision: 3, ChapterIndex: 2, Title: "Chapter", Blocks: []processor.ProseBlock{
		{Kind: "text", Text: "before"},
		{Kind: processor.ProseBlockImage, Src: "https://source.test/image"},
		{Kind: "text", Text: "after"},
	}}
	content, err := p.document(entry, "bundle", false)
	if err != nil {
		t.Fatal(err)
	}
	blocks := content.Document.Blocks
	if len(blocks) != 3 || blocks[0].Kind != "paragraph" || blocks[2].Kind != "paragraph" || blocks[1].Resource == nil || blocks[1].Resource.Href != "resource:book:3:2:0" || content.OfflineCopy {
		t.Fatalf("content=%+v", content)
	}
}

func TestImageAdmissionFailurePreservesTextButNotCacheEligibility(t *testing.T) {
	p := BookSource{}
	entry := book.CachedChapter{Title: "Chapter", Blocks: []processor.ProseBlock{{Kind: "text", Text: "Readable prose"}, {Kind: processor.ProseBlockImage, Src: "private"}}}
	content, err := p.document(entry, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if content.Document.Blocks[0].Text != "Readable prose" || !content.Document.Blocks[1].Resource.Unavailable || content.Document.Blocks[1].Resource.Href != "" || content.FreshForMS == nil || *content.FreshForMS != 0 {
		t.Fatalf("partial document: %+v", content)
	}
	entry.Blocks = entry.Blocks[1:]
	if _, err := p.document(entry, "", true); err != ErrImageUnavailable {
		t.Fatalf("image-only failure: %v", err)
	}
}
