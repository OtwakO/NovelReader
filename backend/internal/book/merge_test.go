package book

import "testing"

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
