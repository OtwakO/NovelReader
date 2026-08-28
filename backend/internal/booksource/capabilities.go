package booksource

import (
	"regexp"
	"strings"
)

var (
	javaBridgePattern    = regexp.MustCompile(`(?i)java\.[a-z]`)
	webViewOptionPattern = regexp.MustCompile(`(?i)\\?["']webView\\?["']\s*:\s*true`)
)

// CapabilityTags returns compact, user-facing execution capabilities for a source definition.
func CapabilityTags(source BookSource) []string {
	definition, _ := source.MarshalJSON()
	text := string(definition)
	tags := make([]string, 0, 5)
	if strings.TrimSpace(source.SearchURL) != "" {
		tags = append(tags, "search")
	}
	if source.EnabledExplore && strings.TrimSpace(source.ExploreURL) != "" {
		tags = append(tags, "explore")
	}
	if strings.TrimSpace(source.Header) != "" {
		tags = append(tags, "headers")
	}
	if strings.TrimSpace(source.JSLib) != "" || strings.Contains(strings.ToLower(text), "<js>") || strings.Contains(strings.ToLower(text), "@js:") || javaBridgePattern.MatchString(text) {
		tags = append(tags, "javascript")
	}
	if webViewOptionPattern.MatchString(text) {
		tags = append(tags, "webview")
	}
	return tags
}
