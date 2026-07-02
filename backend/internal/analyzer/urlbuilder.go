package analyzer

import (
	"fmt"
	"strings"
)

// BuildURL constructs a request URL from a book source's URL template.
// Handles legado template variables:
//   - {{key}} — search keyword
//   - {{page}} — page number
//   - @js: — inline JS evaluation for URL construction
//   - ,{...} — per-URL headers appended after comma
func BuildURL(template, key string, page int, baseURL string, jsVM *JSVM) (string, map[string]string, error) {
	if template == "" {
		return "", nil, nil
	}

	urlStr := template

	// Extract per-URL headers: URL,{...header JSON...}
	extraHeaders := make(map[string]string)
	if idx := strings.Index(urlStr, ",{"); idx != -1 {
		headerPart := urlStr[idx+1:]
		urlStr = urlStr[:idx]
		headerPart = strings.TrimPrefix(headerPart, "{")
		headerPart = strings.TrimSuffix(headerPart, "}")
		for _, pair := range strings.Split(headerPart, ",") {
			pair = strings.TrimSpace(pair)
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) == 2 {
				k := strings.Trim(strings.TrimSpace(kv[0]), "'\"")
				v := strings.Trim(strings.TrimSpace(kv[1]), "'\"")
				extraHeaders[k] = v
			}
		}
	}

	// @js: URL construction
	if strings.HasPrefix(urlStr, "@js:") {
		jsCode := urlStr[4:]
		if jsVM != nil {
			v, err := jsVM.Eval(jsCode, "", baseURL)
			if err == nil {
				urlStr = ToString(v)
			}
		}
	}

	// Replace {{key}} and {{page}}
	urlStr = strings.ReplaceAll(urlStr, "{{key}}", key)
	urlStr = strings.ReplaceAll(urlStr, "{{page}}", fmt.Sprintf("%d", page))

	// Make relative URLs absolute
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "@js:") {
		base := strings.TrimRight(baseURL, "/")
		path := strings.TrimLeft(urlStr, "/")
		urlStr = base + "/" + path
	}

	return urlStr, extraHeaders, nil
}
