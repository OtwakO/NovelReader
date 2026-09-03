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
	definitionFields := []string{
		source.BookURLPattern, source.SearchURL, source.ExploreURL, source.ExploreScreen,
		source.RuleSearch, source.RuleBookInfo, source.RuleToc, source.RuleContent,
		source.RuleExplore, source.RuleReview, source.JSLib, source.Header,
		source.LoginURL, source.LoginUI, source.LoginCheckJS, source.CoverDecodeJS,
	}
	hasJavaScript := strings.TrimSpace(source.JSLib) != ""
	hasWebView := false
	for _, field := range definitionFields {
		lower := strings.ToLower(field)
		if !hasJavaScript && (strings.Contains(lower, "<js>") || strings.Contains(lower, "@js:") || strings.Contains(lower, "java.") && javaBridgePattern.MatchString(field)) {
			hasJavaScript = true
		}
		if !hasWebView && strings.Contains(lower, "webview") && webViewOptionPattern.MatchString(field) {
			hasWebView = true
		}
		if hasJavaScript && hasWebView {
			break
		}
	}
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
	if hasJavaScript {
		tags = append(tags, "javascript")
	}
	if hasWebView {
		tags = append(tags, "webview")
	}
	return tags
}
