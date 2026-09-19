package epub

import (
	"context"
	"errors"
	"fmt"
	"math"
)

var ErrPreparedSection = errors.New("epub: invalid prepared section")

// Add validates one decoded section and collects target evidence in the same
// walk. The caller must first bound untrusted JSON decoding.
func (p *PreparedPublicationCheck) Add(ctx context.Context, section PreparedSection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if section.Ordinal != len(p.placeholders) {
		return fmt.Errorf("%w: section ordinal mismatch", ErrPreparedSection)
	}
	if len(p.placeholders) >= maxTargetSections {
		return ErrLimit
	}
	if err := p.addTarget(p.anchors, SectionTarget{Section: section.Ordinal}); err != nil {
		return err
	}
	for key, image := range section.Images {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !opaqueKey(key) {
			return ErrPreparedSection
		}
		mode := OriginalImages
		if image.DerivativeID != "" {
			mode = OptimizedImages
		}
		if err := ValidatePreparedImage(image, mode); err != nil {
			return err
		}
	}
	ids := make(map[string]bool)
	count := 0
	var visit func(Node, int) error
	visit = func(node Node, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if depth > maxXMLDepth || count > maxXMLTokens {
			return ErrLimit
		}
		if err := validatePreparedNode(node, section.Images); err != nil {
			return fmt.Errorf("%w: node %d", err, count)
		}
		if node.ID != "" {
			if !opaqueKey(node.ID) || ids[node.ID] {
				return fmt.Errorf("%w: duplicate or invalid anchor", ErrPreparedSection)
			}
			ids[node.ID] = true
			if err := p.addTarget(p.anchors, SectionTarget{Section: section.Ordinal, Anchor: node.ID}); err != nil {
				return err
			}
		}
		if node.Target != nil {
			if err := p.addTarget(p.references, *node.Target); err != nil {
				return err
			}
		}
		for _, child := range node.Children {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(section.Root, 1); err != nil {
		return err
	}
	if hasReadableContent(section.Root) == section.CoverPlaceholder {
		return fmt.Errorf("%w: readability contradicts placeholder", ErrPreparedSection)
	}
	p.placeholders = append(p.placeholders, section.CoverPlaceholder)
	return ctx.Err()
}

func validatePreparedNode(node Node, images map[string]PreparedImage) error {
	switch node.Kind {
	case "group", "inlineGroup", "paragraph", "quote", "figure", "figureCaption",
		"unorderedList", "orderedList", "listItem", "table", "tableHead", "tableBody", "tableFoot", "row", "caption",
		"emphasis", "strong", "strike", "subscript", "superscript", "code", "preformatted",
		"break", "separator", "ruby", "rubyText", "rubyFallback", "unsupported", "text", "heading", "cell", "headerCell", "link", "image":
	default:
		return ErrPreparedSection
	}
	if node.Direction != "" && node.Direction != "ltr" && node.Direction != "rtl" && node.Direction != "auto" {
		return ErrPreparedSection
	}
	if node.Role != "" && node.Role != "footnote" && node.Role != "endnote" && node.Role != "noteref" {
		return ErrPreparedSection
	}
	if node.Link != "" {
		return ErrPreparedSection
	} // Private unresolved links must not survive preparation.
	if node.Kind == "text" {
		if len(node.Children) != 0 {
			return ErrPreparedSection
		}
	} else if node.Text != "" {
		return ErrPreparedSection
	}
	if node.Kind == "heading" {
		if node.Level < 1 || node.Level > 6 {
			return ErrPreparedSection
		}
	} else if node.Level != 0 {
		return ErrPreparedSection
	}
	if node.Start != nil && (node.Kind != "orderedList" || int64(*node.Start) < math.MinInt32 || int64(*node.Start) > math.MaxInt32) {
		return ErrPreparedSection
	}
	if node.ColSpan < 0 || node.ColSpan > maxCellSpan || node.RowSpan < 0 || node.RowSpan > maxCellSpan {
		return ErrPreparedSection
	}
	if node.Kind != "cell" && node.Kind != "headerCell" && (node.ColSpan != 0 || node.RowSpan != 0) {
		return ErrPreparedSection
	}
	if node.Kind == "link" {
		actions := 0
		if node.Unavailable {
			actions++
		}
		if node.Target != nil {
			actions++
			if node.Target.Section < 0 || (node.Target.Anchor != "" && !opaqueKey(node.Target.Anchor)) {
				return ErrPreparedSection
			}
		}
		if node.URL != "" {
			actions++
			if _, ok := externalLink(node.URL); !ok {
				return ErrPreparedSection
			}
		}
		if actions != 1 {
			return ErrPreparedSection
		}
	} else if node.Target != nil || node.URL != "" || node.Unavailable {
		return ErrPreparedSection
	}
	if node.Kind == "image" {
		image, exists := images[node.Image]
		if !exists || node.Width != image.Info.Width || node.Height != image.Info.Height {
			return ErrPreparedSection
		}
	} else if node.Image != "" || node.Width != 0 || node.Height != 0 {
		return ErrPreparedSection
	}
	return nil
}

func opaqueKey(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}
