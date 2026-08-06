package processor

import "testing"

func TestProcessPreservesOrderedTextAndImageBlocks(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", `<p>before</p><img src="/one.jpg,{&quot;headers&quot;:{&quot;Referer&quot;:&quot;x&quot;}}"><p>after</p><img src="https://cdn.test/two.png">`)
	if len(result.Blocks) != 4 {
		t.Fatalf("blocks=%+v", result.Blocks)
	}
	if len(result.Paragraphs) != 2 ||
		result.Blocks[0].Type != "text" || result.Blocks[0].Text != result.Paragraphs[0] ||
		result.Blocks[1].Type != "image" || result.Blocks[1].Src != `/one.jpg,{"headers":{"Referer":"x"}}` ||
		result.Blocks[2].Type != "text" || result.Blocks[2].Text != result.Paragraphs[1] ||
		result.Blocks[3].Type != "image" || result.Blocks[3].Src != "https://cdn.test/two.png" {
		t.Fatalf("paragraphs=%+v blocks=%+v", result.Paragraphs, result.Blocks)
	}
}

func TestProcessOmitsBlocksForTextOnlyContent(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", "first\nsecond")
	if result.Blocks != nil {
		t.Fatalf("blocks=%+v", result.Blocks)
	}
}
