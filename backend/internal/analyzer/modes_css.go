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
		switch attr {
		case "html":
			v, _ := selection.Html()
			return v, nil
		case "text":
			return strings.TrimSpace(selection.Text()), nil
		case "ownText":
			return strings.Join(cssGetterValues(selection, ownText), "\n"), nil
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
	var results []string
	doc.Find(sel).Each(func(i int, s *goquery.Selection) {
		var v string
		if attr != "" {
			switch attr {
			case "html":
				v, _ = s.Html()
			case "text":
				v = s.Text()
			case "ownText":
				v = ownText(s)
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
	selection := doc.Find(sel)
	var results []interface{}
	for i := 0; i < selection.Length(); i++ {
		h, _ := goquery.OuterHtml(selection.Eq(i))
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
