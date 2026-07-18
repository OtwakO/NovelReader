// Source-switch tests keep canonical progress on readable target chapters.
package book

import "testing"

func TestMigrateChapterIndexMatchesNormalizedTitleThenClampsRawIndex(t *testing.T) {
	chapters := []Chapter{
		{Index: 0, Title: "Volume", IsVolume: true},
		{Index: 1, Title: "第 12 章：风雨！"},
		{Index: 2, Title: "After"},
	}
	if index, mapping := MigrateChapterIndex(chapters, "第12章 风雨", 2); index != 1 || mapping != "title" {
		t.Fatalf("title index=%d mapping=%q", index, mapping)
	}
	if index, mapping := MigrateChapterIndex(chapters, "Missing", 99); index != 2 || mapping != "index" {
		t.Fatalf("fallback index=%d mapping=%q", index, mapping)
	}
	if index, mapping := MigrateChapterIndex(chapters, "Missing", 0); index != 1 || mapping != "index" {
		t.Fatalf("volume fallback index=%d mapping=%q", index, mapping)
	}
	if index, mapping := MigrateChapterIndex([]Chapter{{Index: 0, IsVolume: true}}, "Missing", 0); index != -1 || mapping != "" {
		t.Fatalf("unreadable index=%d mapping=%q", index, mapping)
	}
}
