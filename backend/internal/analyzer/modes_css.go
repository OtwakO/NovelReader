package analyzer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// cssQuery extracts text content using a CSS selector.
// Supports @attr suffix for attribute extraction.
// If no @attr, returns text content.
func cssQuery(html, selector string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("css: parse: %w", err)
	}

	sel, attr := splitCSSAttr(selector)
	selection := doc.Find(translateJsoupPositionalSelectors(sel))
	if selection.Length() == 0 {
		return "", nil
	}

	if attr != "" {
		switch attr {
		case "html":
			return cssOuterHTML(selection, true), nil
		case "all":
			return cssOuterHTML(selection, false), nil
		case "text":
			return strings.TrimSpace(selection.Text()), nil
		case "ownText":
			return strings.Join(cssGetterValues(selection, ownText), "\n"), nil
		case "textNodes":
			return strings.Join(cssGetterValues(selection, directTextNodes), "\n"), nil
		}
		seen := make(map[string]struct{}, selection.Length())
		values := make([]string, 0, selection.Length())
		selection.Each(func(_ int, item *goquery.Selection) {
			value, _ := item.Attr(attr)
			value = strings.TrimSpace(value)
			if value == "" {
				return
			}
			if _, duplicate := seen[value]; duplicate {
				return
			}
			seen[value] = struct{}{}
			values = append(values, value)
		})
		return strings.Join(values, "\n"), nil
	}
	return strings.TrimSpace(selection.First().Text()), nil
}

// cssQueryList extracts a list of text values from matching elements.
func cssQueryList(html, selector string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("css: parse: %w", err)
	}

	sel, attr := splitCSSAttr(selector)
	selection := doc.Find(translateJsoupPositionalSelectors(sel))
	if attr == "html" || attr == "all" {
		value := cssOuterHTML(selection, attr == "html")
		if value == "" {
			return nil, nil
		}
		return []string{value}, nil
	}

	var results []string
	selection.Each(func(i int, s *goquery.Selection) {
		var v string
		if attr != "" {
			switch attr {
			case "text":
				v = s.Text()
			case "ownText":
				v = ownText(s)
			case "textNodes":
				v = directTextNodes(s)
			default:
				v, _ = s.Attr(attr)
			}
		} else {
			v = s.Text()
		}
		v = strings.TrimSpace(v)
		if v != "" {
			results = append(results, v)
		}
	})
	return results, nil
}

func cssOuterHTML(selection *goquery.Selection, removeScriptsAndStyles bool) string {
	if removeScriptsAndStyles {
		selection.Find("script, style").Remove()
	}
	values := make([]string, 0, selection.Length())
	selection.Each(func(_ int, item *goquery.Selection) {
		if value, err := goquery.OuterHtml(item); err == nil && value != "" {
			values = append(values, value)
		}
	})
	return strings.Join(values, "\n")
}

func directTextNodes(selection *goquery.Selection) string {
	var values []string
	selection.Contents().Each(func(_ int, child *goquery.Selection) {
		if goquery.NodeName(child) != "#text" {
			return
		}
		if value := strings.TrimSpace(child.Text()); value != "" {
			values = append(values, value)
		}
	})
	return strings.Join(values, "\n")
}

func cssGetterValues(selection *goquery.Selection, getter func(*goquery.Selection) string) []string {
	values := make([]string, 0, selection.Length())
	selection.Each(func(_ int, item *goquery.Selection) {
		if value := strings.TrimSpace(getter(item)); value != "" {
			values = append(values, value)
		}
	})
	return values
}

// cssQueryElements returns elements as interface{} for further chaining.
// Returns outer HTML so field rules can match the selected element itself.
func cssQueryElements(html, selector string) ([]interface{}, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("css: parse: %w", err)
	}

	sel, _ := splitCSSAttr(selector)
	selection := doc.Find(translateJsoupPositionalSelectors(sel))
	var results []interface{}
	for i := 0; i < selection.Length(); i++ {
		h, _ := goquery.OuterHtml(selection.Eq(i))
		results = append(results, h)
	}
	return results, nil
}

var jsoupPositionalSelectorPattern = regexp.MustCompile(`(?i)^:(eq|lt|gt)\(\s*([+-]?\d+)\s*\)`)

func translateJsoupPositionalSelectors(selector string) string {
	var translated strings.Builder
	translated.Grow(len(selector))
	attributeDepth := 0
	var quote byte

	for i := 0; i < len(selector); {
		char := selector[i]
		if quote != 0 {
			translated.WriteByte(char)
			i++
			if char == '\\' && i < len(selector) {
				translated.WriteByte(selector[i])
				i++
			} else if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			translated.WriteByte(char)
			i++
			continue
		}
		switch char {
		case '[':
			attributeDepth++
		case ']':
			if attributeDepth > 0 {
				attributeDepth--
			}
		}
		if attributeDepth == 0 {
			if parts := jsoupPositionalSelectorPattern.FindStringSubmatch(selector[i:]); parts != nil {
				translated.WriteString(translateJsoupPositionalSelector(parts[0], parts[1], parts[2]))
				i += len(parts[0])
				continue
			}
		}
		translated.WriteByte(char)
		i++
	}
	return translated.String()
}

func translateJsoupPositionalSelector(match, operation, indexText string) string {
	index, err := strconv.Atoi(indexText)
	if err != nil {
		return match
	}
	switch strings.ToLower(operation) {
	case "eq":
		if index < 0 {
			return ":not(*)"
		}
		return fmt.Sprintf(":nth-child(%d)", index+1)
	case "lt":
		if index <= 0 {
			return ":not(*)"
		}
		return fmt.Sprintf(":nth-child(-n+%d)", index)
	case "gt":
		if index < 0 {
			return ""
		}
		return fmt.Sprintf(":nth-child(n+%d)", index+2)
	default:
		return match
	}
}

// splitCSSAttr splits "tag@attr" into selector and attribute name.
func splitCSSAttr(selector string) (string, string) {
	// First check for @attr suffix (but not @html or @text — those are special)
	if idx := strings.LastIndex(selector, "@"); idx != -1 {
		attr := selector[idx+1:]
		sel := selector[:idx]
		// Handle escaped @ in attribute values (unlikely in practice)
		if strings.Contains(attr, "\"") || strings.Contains(attr, "'") {
			return selector, ""
		}
		return sel, attr
	}
	return selector, ""
}
