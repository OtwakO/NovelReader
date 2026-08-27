package processor

import (
	"reflect"
	"testing"
)

func TestProcessSpacesInlineCJKClosingQuote(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", "“你家已经困难到这个地步了”欧阳小声问秦淮")
	want := []string{"“你家已经困难到这个地步了” 欧阳小声问秦淮"}
	if !reflect.DeepEqual(result.Paragraphs, want) {
		t.Fatalf("paragraphs=%q want=%q", result.Paragraphs, want)
	}
}

func TestProcessRejoinsExtractedOrphanClosingQuoteWithOneSpace(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", "“你家已经困难到这个地步了\n\n　　”欧阳小声问秦淮")
	want := []string{"“你家已经困难到这个地步了” 欧阳小声问秦淮"}
	if !reflect.DeepEqual(result.Paragraphs, want) {
		t.Fatalf("paragraphs=%q want=%q", result.Paragraphs, want)
	}
}

func TestProcessDoesNotAddSpaceAfterClosingQuoteBeforePunctuation(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", "他说：“好。”然后离开。\n他说：“好。”，然后离开。")
	want := []string{"他说：“好。” 然后离开。", "　　他说：“好。”，然后离开。"}
	if !reflect.DeepEqual(result.Paragraphs, want) {
		t.Fatalf("paragraphs=%q want=%q", result.Paragraphs, want)
	}
}

func TestProcessPreservesSentencePunctuation(t *testing.T) {
	result := New(DefaultConfig()).Process("Chapter", "你还好吗？\n我很好！\n这是句号。\nKeep this?")
	want := []string{
		"你还好吗？",
		"　　我很好！",
		"　　这是句号。",
		"　　Keep this?",
	}
	if !reflect.DeepEqual(result.Paragraphs, want) {
		t.Fatalf("paragraphs=%q want=%q", result.Paragraphs, want)
	}
}
