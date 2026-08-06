// Package processor handles chapter content cleanup: replace rules, Chinese conversion, paragraph formatting.
package processor

import (
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

// ChineseConvertMode defines simplified/traditional conversion.
type ChineseConvertMode int

const (
	ConvertNone ChineseConvertMode = iota
	SToT                           // Simplified → Traditional
	TToS                           // Traditional → Simplified
)

// Config holds content processing settings.
type Config struct {
	ChineseConvert       ChineseConvertMode
	ParagraphIndent      string // default: two spaces or \u3000\u3000
	RemoveDuplicateTitle bool
	ReSegment            bool
	UseReplaceRules      bool
}

// DefaultConfig returns sensible defaults for reading.
func DefaultConfig() Config {
	return Config{
		ParagraphIndent:      "\u3000\u3000", // two full-width spaces
		RemoveDuplicateTitle: true,
		ReSegment:            true,
		UseReplaceRules:      true,
	}
}

// ReplaceRule is a single text replacement rule.
type ReplaceRule struct {
	Name        string
	Pattern     string // regex or literal pattern
	Replacement string
	IsRegex     bool
	Enabled     bool
}

// ProcessResult holds the processed content and metadata.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Src  string `json:"src,omitempty"`
}

type ProcessResult struct {
	Title      string         `json:"title"`
	Paragraphs []string       `json:"paragraphs"`
	Blocks     []ContentBlock `json:"blocks,omitempty"`
	// ponytail: ReplaceRulesEffective is omitted — only add when the UI needs to show it.
}

// ContentProcessor cleans and formats raw chapter content.
type ContentProcessor struct {
	titleReplaceRules   []ReplaceRule
	contentReplaceRules []ReplaceRule
	config              Config
}

func New(config Config) *ContentProcessor {
	return &ContentProcessor{config: config}
}

// Process runs all processing steps on raw content.
func (p *ContentProcessor) Process(title, rawContent string) ProcessResult {
	paragraphs := p.processParagraphs(title, rawContent)
	return ProcessResult{
		Title:      title,
		Paragraphs: paragraphs,
		Blocks:     p.toBlocks(title, rawContent),
	}
}

func (p *ContentProcessor) processParagraphs(title, content string) []string {
	if p.config.RemoveDuplicateTitle && content != "null" {
		content = p.removeTitle(title, content)
	}
	if p.config.ReSegment {
		content = p.reSegment(content, title)
	}
	if p.config.ChineseConvert != ConvertNone {
		content = p.convertChinese(content, p.config.ChineseConvert)
	}
	if p.config.UseReplaceRules && len(p.contentReplaceRules) > 0 {
		content = p.applyReplaceRules(content, p.contentReplaceRules)
	}
	return p.toParagraphs(title, content)
}

func (p *ContentProcessor) toBlocks(title, content string) []ContentBlock {
	if !strings.Contains(strings.ToLower(content), "<img") {
		return nil
	}
	const markerPrefix = "NOVELREADERIMAGEBLOCK"
	var marked strings.Builder
	var sources []string
	tokenizer := xhtml.NewTokenizer(strings.NewReader(content))
	for {
		tokenType := tokenizer.Next()
		if tokenType == xhtml.ErrorToken {
			if tokenizer.Err() != nil && tokenizer.Err() != io.EOF {
				return nil
			}
			break
		}
		if tokenType == xhtml.StartTagToken || tokenType == xhtml.SelfClosingTagToken {
			token := tokenizer.Token()
			if strings.EqualFold(token.Data, "img") {
				for _, attribute := range token.Attr {
					if strings.EqualFold(attribute.Key, "src") && strings.TrimSpace(attribute.Val) != "" {
						index := len(sources)
						sources = append(sources, strings.TrimSpace(attribute.Val))
						fmt.Fprintf(&marked, "<br>%s%dTOKEN<br>", markerPrefix, index)
						break
					}
				}
				continue
			}
		}
		marked.Write(tokenizer.Raw())
	}
	if len(sources) == 0 {
		return nil
	}
	paragraphs := p.processParagraphs(title, marked.String())
	blocks := make([]ContentBlock, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		trimmed := strings.TrimSpace(paragraph)
		if strings.HasPrefix(trimmed, markerPrefix) && strings.HasSuffix(trimmed, "TOKEN") {
			var index int
			if _, err := fmt.Sscanf(trimmed, markerPrefix+"%dTOKEN", &index); err == nil && index >= 0 && index < len(sources) {
				blocks = append(blocks, ContentBlock{Type: "image", Src: sources[index]})
				continue
			}
		}
		blocks = append(blocks, ContentBlock{Type: "text", Text: paragraph})
	}
	return blocks
}

func (p *ContentProcessor) removeTitle(title, content string) string {
	// Escape regex special chars in title
	quoted := regexp.QuoteMeta(title)
	// Match: start of string, optional whitespace/punctuation/book name, then title, then whitespace
	pattern := regexp.MustCompile(`^(\s|\p{P}|` + quoted + `)*` + quoted + `(\s)*`)
	if loc := pattern.FindStringIndex(content); loc != nil {
		return content[loc[1]:]
	}
	return content
}

func (p *ContentProcessor) reSegment(content, title string) string {
	// Split on common Chinese punctuation (. 。! ！? ？) followed by whitespace or newline
	re := regexp.MustCompile(`([。！？.!?\n])`)
	segments := re.Split(content, -1)

	var result []string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg != "" {
			result = append(result, seg)
		}
	}
	return strings.Join(result, "\n")
}

func (p *ContentProcessor) convertChinese(content string, mode ChineseConvertMode) string {
	// ponytail: Chinese conversion not yet implemented. Wire opencc or golang.org/x/text when needed.
	return content
}

func (p *ContentProcessor) applyReplaceRules(content string, rules []ReplaceRule) string {
	for _, rule := range rules {
		if !rule.Enabled || rule.Pattern == "" {
			continue
		}
		if rule.IsRegex {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				continue
			}
			content = re.ReplaceAllString(content, rule.Replacement)
		} else {
			content = strings.ReplaceAll(content, rule.Pattern, rule.Replacement)
		}
	}
	return content
}

func (p *ContentProcessor) toParagraphs(title, content string) []string {
	// Strip HTML tags, convert <br>/<p> to newlines
	content = stripHTMLToNewlines(content)
	// Unescape HTML entities
	content = html.UnescapeString(content)

	lines := strings.Split(content, "\n")
	var paragraphs []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		trimmed := trimSpacePunct(line)
		if trimmed == "" {
			continue
		}
		if i == 0 {
			paragraphs = append(paragraphs, trimmed)
		} else {
			paragraphs = append(paragraphs, p.config.ParagraphIndent+trimmed)
		}
	}

	if len(paragraphs) == 0 {
		paragraphs = append(paragraphs, title)
	}
	return paragraphs
}

// stripHTMLToNewlines converts block-level HTML tags to newlines and strips all remaining tags.
// This prevents XSS via {@html} in the frontend reader.
func stripHTMLToNewlines(s string) string {
	// Convert common block tags to newlines
	repl := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n", "</div>", "\n", "</li>", "\n",
		"</tr>", "\n", "</h1>", "\n", "</h2>", "\n",
		"</h3>", "\n", "</h4>", "\n", "</h5>", "\n", "</h6>", "\n",
	)
	s = repl.Replace(s)

	// Strip remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]*>`)
	s = tagRe.ReplaceAllString(s, "")

	// Collapse multiple newlines
	multiNewline := regexp.MustCompile(`\n{3,}`)
	s = multiNewline.ReplaceAllString(s, "\n\n")

	return s
}

func trimSpacePunct(s string) string {
	// Remove leading whitespace and full-width spaces
	s = strings.TrimLeft(s, " \t\n\r\u00a0\u3000")
	s = strings.TrimRight(s, " \t\n\r\u00a0\u3000")
	return s
}

// ponytail: Chinese conversion maps removed. Add opencc or golang.org/x/text when needed.
