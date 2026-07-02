package analyzer

import (
	"fmt"
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
	selection := doc.Find(sel)
	if selection.Length() == 0 {
		return "", nil
	}

	if attr != "" {
		if attr == "html" {
			v, _ := selection.Html()
			return v, nil
		}
		if attr == "text" {
			return strings.TrimSpace(selection.Text()), nil
		}
		v, _ := selection.Attr(attr)
		return v, nil
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
	var results []string
	doc.Find(sel).Each(func(i int, s *goquery.Selection) {
		var v string
		if attr != "" {
			if attr == "html" {
				v, _ = s.Html()
			} else if attr == "text" {
				v = s.Text()
			} else {
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

// cssQueryElements returns elements as interface{} for further chaining.
// ponytail: caps at 50 to avoid wasting parse work on huge lists.
func cssQueryElements(html, selector string) ([]interface{}, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("css: parse: %w", err)
	}

	sel, _ := splitCSSAttr(selector)
	selection := doc.Find(sel)
	total := selection.Length()
	limit := total
	if limit > 50 {
		limit = 50
	}
	var results []interface{}
	for i := 0; i < limit; i++ {
		h, _ := selection.Eq(i).Html()
		results = append(results, h)
	}
	return results, nil
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
