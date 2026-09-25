package config

import (
	"testing"

	"github.com/otwako/novelreader/internal/chapterresource"
)

func TestChapterResourceEnvironment(t *testing.T) {
	keys := []string{"CHAPTER_RESOURCE_CACHE_MAX_MIB", "CHAPTER_RESOURCE_CACHE_READER_MAX_MIB", "CHAPTER_RESOURCE_CACHE_MAX_BUNDLES", "CHAPTER_RESOURCE_CACHE_READER_MAX_BUNDLES"}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	if got := Load().ChapterResourceCache; got != chapterresource.DefaultLimits() {
		t.Fatalf("defaults: %+v", got)
	}
	values := []string{"2048", "512", "20000", "4000"}
	for i, key := range keys {
		t.Setenv(key, values[i])
	}
	want := chapterresource.Limits{TotalBytes: 2 << 30, ReaderBytes: 512 << 20, TotalBundles: 20000, ReaderBundles: 4000}
	if got := Load().ChapterResourceCache; got != want {
		t.Fatalf("overrides: %+v", got)
	}
	values = []string{"9223372036854775807", "-1", "invalid", "0"}
	for i, key := range keys {
		t.Setenv(key, values[i])
	}
	if got := Load().ChapterResourceCache; got != chapterresource.DefaultLimits() {
		t.Fatalf("invalid-value fallback: %+v", got)
	}
}
