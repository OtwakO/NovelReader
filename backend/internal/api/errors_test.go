package api

import "testing"

func TestPublicCrawlURLHidesOpaqueDataDocuments(t *testing.T) {
	if got := publicCrawlURL("data:;base64,eyJib29rIjoidGVzdCJ9"); got != "source-provided data" {
		t.Fatalf("got %q", got)
	}
	const publicURL = "https://example.com/toc/2"
	if got := publicCrawlURL(publicURL); got != publicURL {
		t.Fatalf("got %q", got)
	}
}
