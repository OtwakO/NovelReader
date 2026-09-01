package book

import "testing"

func TestMergeAndSortKeepsLastChapterOnAlternateBinding(t *testing.T) {
	results := MergeAndSort("Book", []SearchResult{
		{Name: "Book", Author: "Author", SourceID: "a", SourceURL: "a", BookURL: "/a", SourceName: "Aggregate", LastChapter: "Provider A"},
		{Name: "Book", Author: "Author", SourceID: "b", SourceURL: "b", BookURL: "/b", SourceName: "Aggregate", LastChapter: "Provider B"},
	})
	if len(results) != 1 || len(results[0].AlternateSources) != 1 {
		t.Fatalf("results=%+v", results)
	}
	if results[0].AlternateSources[0].LastChapter == "" || results[0].AlternateSources[0].LastChapter == results[0].LastChapter {
		t.Fatalf("binding display snapshots were not preserved: %+v", results[0])
	}
}

func TestScoreResultRanksTitleBeforeAuthorAtSameSpecificity(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		title  string
		author string
		want   int
	}{
		{name: "exact title", query: "忘语", title: "忘语", author: "其他作者", want: 100},
		{name: "exact normalized author", query: "忘语", title: "凡人修仙传", author: "作者：忘语", want: 90},
		{name: "title prefix", query: "忘语", title: "忘语作品集", author: "其他作者", want: 80},
		{name: "author prefix", query: "忘语", title: "凡人修仙传", author: "忘语作品", want: 70},
		{name: "title contains", query: "忘语", title: "寻找忘语", author: "其他作者", want: 60},
		{name: "author contains", query: "忘语", title: "凡人修仙传", author: "作家忘语", want: 50},
		{name: "source supplied fuzzy result", query: "忘语", title: "凡人修仙传", author: "其他作者", want: 20},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := scoreResult(test.query, test.title, test.author); got != test.want {
				t.Fatalf("scoreResult(%q, %q, %q) = %d, want %d", test.query, test.title, test.author, got, test.want)
			}
		})
	}
}
