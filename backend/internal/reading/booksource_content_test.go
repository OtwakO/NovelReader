package reading

import (
	"fmt"
	"testing"

	"github.com/otwako/novelreader/internal/processor"
)

func TestBookSourceDocumentNormalizesCacheBlocksAndIssuesResources(t *testing.T) {
	p := BookSource{ImageHref: func(id string, revision int64, chapter, image int) string {
		return fmt.Sprintf("resource:%s:%d:%d:%d", id, revision, chapter, image)
	}}
	content := p.content("book", 3, 2, "Chapter", nil, []processor.ProseBlock{
		{Kind: "text", Text: "before"},
		{Kind: processor.ProseBlockImage, Src: "https://source.test/image"},
		{Kind: "text", Text: "after"},
	}, true)
	blocks := content.Document.Blocks
	if len(blocks) != 3 || blocks[0].Kind != "paragraph" || blocks[2].Kind != "paragraph" || blocks[1].Resource == nil || blocks[1].Resource.Href != "resource:book:3:2:0" || !content.OfflineCopy {
		t.Fatalf("content=%+v", content)
	}
}
