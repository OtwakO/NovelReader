package book

import (
	"html"
	"strings"

	htmltokenizer "golang.org/x/net/html"
)

var descriptionBlockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"div": true, "dl": true, "dt": true, "dd": true, "figcaption": true,
	"figure": true, "footer": true, "form": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "header": true,
	"hr": true, "li": true, "main": true, "nav": true, "ol": true,
	"p": true, "pre": true, "section": true, "table": true, "tbody": true,
	"td": true, "tfoot": true, "th": true, "thead": true, "tr": true,
	"ul": true,
}

// NormalizeDescription converts untrusted source markup into display-safe plain
// prose while preserving source-authored spacing and paragraph boundaries.
func NormalizeDescription(value string) string {
	// Decode before tokenizing so entity-encoded tags are treated as markup and
	// stripped at the same trust boundary. A bounded second pass handles common
	// double-encoded source payloads without an unbounded decode loop.
	for range 2 {
		decoded := html.UnescapeString(value)
		if decoded == value {
			break
		}
		value = decoded
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")

	tokenizer := htmltokenizer.NewTokenizer(strings.NewReader(value))
	var output strings.Builder
	skipDepth := 0

	appendBoundary := func() {
		if output.Len() == 0 {
			return
		}
		text := output.String()
		if text[len(text)-1] != '\n' {
			output.WriteByte('\n')
		}
	}

	for {
		tokenType := tokenizer.Next()
		if tokenType == htmltokenizer.ErrorToken {
			break
		}
		token := tokenizer.Token()
		tag := strings.ToLower(token.Data)
		switch tokenType {
		case htmltokenizer.StartTagToken:
			if tag == "script" || tag == "style" {
				skipDepth++
				continue
			}
			if skipDepth > 0 {
				continue
			}
			if tag == "br" || descriptionBlockTags[tag] {
				appendBoundary()
			}
		case htmltokenizer.SelfClosingTagToken:
			if skipDepth == 0 && (tag == "br" || descriptionBlockTags[tag]) {
				appendBoundary()
			}
		case htmltokenizer.EndTagToken:
			if tag == "script" || tag == "style" {
				if skipDepth > 0 {
					skipDepth--
				}
				continue
			}
			if skipDepth == 0 && descriptionBlockTags[tag] {
				appendBoundary()
			}
		case htmltokenizer.TextToken:
			if skipDepth == 0 {
				output.WriteString(token.Data)
			}
		}
	}

	plain := strings.ReplaceAll(output.String(), "\u00a0", " ")
	lines := strings.Split(plain, "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(lines[index])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
