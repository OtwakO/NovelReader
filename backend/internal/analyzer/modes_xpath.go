package analyzer

import (
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// xpathQuery extracts text using an XPath expression.
func xpathQuery(htmlContent, expr string) (string, error) {
	doc, err := htmlquery.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("xpath: parse: %w", err)
	}

	node, err := htmlquery.Query(doc, expr)
	if err != nil {
		return "", fmt.Errorf("xpath: query: %w", err)
	}
	if node == nil {
		return "", nil
	}
	return extractXPathText(node), nil
}

// xpathQueryList extracts a list of texts using an XPath expression.
func xpathQueryList(htmlContent, expr string) ([]string, error) {
	doc, err := htmlquery.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("xpath: parse: %w", err)
	}

	nodes, err := htmlquery.QueryAll(doc, expr)
	if err != nil {
		return nil, fmt.Errorf("xpath: query all: %w", err)
	}

	var results []string
	for _, node := range nodes {
		v := strings.TrimSpace(extractXPathText(node))
		if v != "" {
			results = append(results, v)
		}
	}
	return results, nil
}

// xpathQueryElements returns matching element HTML snippets, capped at 50.
func xpathQueryElements(htmlContent, expr string) ([]interface{}, error) {
	doc, err := htmlquery.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("xpath: parse: %w", err)
	}

	nodes, err := htmlquery.QueryAll(doc, expr)
	if err != nil {
		return nil, fmt.Errorf("xpath: query all: %w", err)
	}

	limit := len(nodes)
	if limit > 50 {
		limit = 50
	}
	var results []interface{}
	for _, node := range nodes[:limit] {
		var b strings.Builder
		if err := html.Render(&b, node); err == nil {
			results = append(results, b.String())
		}
	}
	return results, nil
}

// extractXPathText gets the text content of an XPath node.
func extractXPathText(n *html.Node) string {
	return htmlquery.InnerText(n)
}
