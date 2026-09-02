package processor

import "testing"

func TestProcessPreservesOrderedTextAndImageBlocks(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", `<p>before</p><img src="/one.jpg,{&quot;headers&quot;:{&quot;Referer&quot;:&quot;x&quot;}}" alt="Map of the northern road"><p>after</p><img src="https://cdn.test/two.png">`)
	if len(result.Blocks) != 4 {
		t.Fatalf("blocks=%+v", result.Blocks)
	}
	if len(result.Paragraphs) != 2 ||
		result.Blocks[0].Kind != ProseBlockParagraph || result.Blocks[0].Text != result.Paragraphs[0] ||
		result.Blocks[1].Kind != ProseBlockImage || result.Blocks[1].Src != `/one.jpg,{"headers":{"Referer":"x"}}` || result.Blocks[1].Alt != "Map of the northern road" ||
		result.Blocks[2].Kind != ProseBlockParagraph || result.Blocks[2].Text != result.Paragraphs[1] ||
		result.Blocks[3].Kind != ProseBlockImage || result.Blocks[3].Src != "https://cdn.test/two.png" || result.Blocks[3].Alt != "" {
		t.Fatalf("paragraphs=%+v blocks=%+v", result.Paragraphs, result.Blocks)
	}
}

func TestProcessCreatesParagraphBlocksForTextOnlyContent(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", "first\nsecond")
	if len(result.Blocks) != 2 || result.Blocks[0].Kind != ProseBlockParagraph || result.Blocks[0].Text != result.Paragraphs[0] || result.Blocks[1].Kind != ProseBlockParagraph || result.Blocks[1].Text != result.Paragraphs[1] {
		t.Fatalf("paragraphs=%+v blocks=%+v", result.Paragraphs, result.Blocks)
	}
}
