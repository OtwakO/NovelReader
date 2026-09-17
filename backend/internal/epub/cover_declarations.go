package epub

import (
	"context"
	"slices"
	"strings"
)

// Only whole-document declarations can exempt an unreadable cover page. A
// fragment landmark may identify just part of a mixed-content document.
func landmarkCoverPages(ctx context.Context, root *xmlElement, base string, p Package) ([]Reference, error) {
	paths := make(map[string]bool, len(p.Items))
	for _, item := range p.Items {
		if item.MediaType == "application/xhtml+xml" {
			paths[item.Reference.Path] = true
		}
	}
	var result []Reference
	var visit func(*xmlElement, bool) error
	visit = func(node *xmlElement, landmark bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if node.is(xhtmlNamespace, "nav") {
			landmark = slices.Contains(strings.Fields(node.attr(epubNamespace, "type")), "landmarks")
		}
		if landmark && node.is(xhtmlNamespace, "a") && slices.Contains(strings.Fields(node.attr(epubNamespace, "type")), "cover") {
			ref, err := resolveReference(base, node.attr("", "href"))
			if err == nil && ref.Fragment == "" && paths[ref.Path] {
				if len(result) >= maxNavigationEntries {
					return ErrLimit
				}
				result = append(result, ref)
			}
		}
		for i := range node.Children {
			if err := visit(&node.Children[i], landmark); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, false); err != nil {
		return nil, err
	}
	return result, nil
}
