package txtstore

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/txt"
)

func TestAnalysisFailureCodes(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		options    txt.Options
		code       string
	}{
		{"uncertain", "\xff", txt.Options{}, "txt_encoding_required"},
		{"malformed", "\xff", txt.Options{Encoding: txt.UTF8}, "txt_invalid_encoding"},
		{"utf32", "\xff\xfe\x00\x00a", txt.Options{}, "txt_unsupported_encoding"},
		{"bom conflict", "\xef\xbb\xbftext", txt.Options{Encoding: txt.Big5}, "txt_invalid_encoding"},
		{"missing bom", "text", txt.Options{Encoding: txt.UTF16LE}, "txt_invalid_encoding"},
		{"truncated utf16", "\xff\xfea", txt.Options{}, "txt_invalid_encoding"},
		{"legacy", "\x81", txt.Options{Encoding: txt.Big5}, "txt_invalid_encoding"},
		{"empty", " \n\t", txt.Options{}, "txt_no_readable_text"},
		{"binary", "a\x00b", txt.Options{}, "txt_non_text"},
		{"section limit", strings.Repeat("Chapter 1\ntext\n", 50001), txt.Options{Preset: txt.EnglishChapters}, "txt_section_limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _, _, _ := receiptStore(t)
			receipt, err := store.Receive(t.Context(), rand.Text(), "sample.txt", strings.NewReader(tc.text))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.Analyze(t.Context(), receipt.ID, tc.options); err == nil {
				t.Fatal("expected failure")
			}
			failed, err := store.Get(t.Context(), receipt.ID)
			if err != nil || failed.State != AnalysisFailed || failed.Error != tc.code {
				t.Fatalf("state=%s code=%q err=%v", failed.State, failed.Error, err)
			}
		})
	}
}
