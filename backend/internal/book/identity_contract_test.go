package book

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSharedBookIdentityContract(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/book-identity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Author string
		Expected     struct{ Name, Author string }
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name+"/"+tc.Author, func(t *testing.T) {
			name, author := NormalizeBookIdentity(tc.Name, tc.Author)
			if name != tc.Expected.Name || author != tc.Expected.Author {
				t.Fatalf("got (%q, %q), want (%q, %q)", name, author, tc.Expected.Name, tc.Expected.Author)
			}
		})
	}
}
