package epub

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const maxSectionBytes = 16 << 20

// NormalizeSection interprets one XHTML document, without fetching its targets
// or images. Preparation must validate those bindings across the publication
// before this result can be read. No archive/library/storage ownership lives here.
func NormalizeSection(ctx context.Context, data []byte, base string) (Section, error) {
	section, err := normalizeSection(ctx, data, base)
	if err == nil && !hasReadableContent(section.Root) {
		return Section{}, fmt.Errorf("%w: no readable content", ErrUnsupported)
	}
	return section, err
}

// Preparation decides admission after image validation, including explicit covers.
func normalizeSection(ctx context.Context, data []byte, base string) (Section, error) {
	if !validEntryName(base) {
		return Section{}, ErrReference
	}
	if len(data) > maxSectionBytes {
		return Section{}, ErrLimit
	}
	var root xmlElement
	if err := decodeXML(ctx, data, &root); err != nil {
		return Section{}, err
	}
	if !root.is(xhtmlNamespace, "html") {
		return Section{}, ErrUnsupported
	}
	if root.hasBaseOverride() {
		return Section{}, fmt.Errorf("%w: content base override", ErrUnsupported)
	}
	body := root.child(xhtmlNamespace, "body")
	if body == nil {
		return Section{}, ErrPackage
	}
	n := sectionNormalizer{ctx: ctx, base: base, section: Section{Anchors: map[string]string{}, Links: map[string]Reference{}, Images: map[string]Reference{}}}
	if head := root.child(xhtmlNamespace, "head"); head != nil {
		if title := head.child(xhtmlNamespace, "title"); title != nil {
			n.section.Title = strings.Join(strings.Fields(title.labelText()), " ")
		}
	}
	node, err := n.node(body, false)
	if err != nil {
		return Section{}, err
	}
	n.section.Cover = slices.Contains(strings.Fields(body.attr(epubNamespace, "type")), "cover")
	n.section.Root = *node
	// The html element's inherited language/direction also applies to body.
	if n.section.Root.Language == "" {
		n.section.Root.Language = elementLanguage(&root)
	}
	if n.section.Root.Direction == "" {
		n.section.Root.Direction = n.direction(root.attr("", "dir"))
	}
	// A document-level ID targets the beginning, not a separate invisible node.
	n.anchors(&root, &n.section.Root)
	if err := ctx.Err(); err != nil {
		return Section{}, err
	}
	return n.section, nil
}

type sectionNormalizer struct {
	ctx         context.Context
	base        string
	section     Section
	anchorCount int
}

func (n *sectionNormalizer) warn(code string) {
	if !slices.Contains(n.section.Diagnostics, code) {
		n.section.Diagnostics = append(n.section.Diagnostics, code)
	}
}

func (n *sectionNormalizer) node(source *xmlElement, pre bool) (*Node, error) {
	if err := n.ctx.Err(); err != nil {
		return nil, err
	}
	if source.Name.Local == "" {
		text := source.Text
		if !pre {
			text = collapseTextSpace(text)
		}
		if text == "" {
			return nil, nil
		}
		return &Node{Kind: "text", Text: text}, nil
	}
	if source.Name.Space != xhtmlNamespace {
		return n.foreignNode(source)
	}
	out := Node{Kind: "group", Language: elementLanguage(source), Direction: n.direction(source.attr("", "dir"))}
	switch source.Name.Local {
	case "script", "style":
		if source.Name.Local == "script" {
			n.warn("active_content_removed")
		}
		return nil, nil
	case "body", "div", "section", "article", "main", "header", "footer", "nav", "aside":
	case "span", "a", "noscript":
		out.Kind = "inlineGroup"
	case "figure":
		out.Kind = "figure"
	case "figcaption":
		out.Kind = "figureCaption"
	case "p":
		out.Kind = "paragraph"
	case "h1", "h2", "h3", "h4", "h5", "h6":
		out.Kind, out.Level = "heading", int(source.Name.Local[1]-'0')
	case "blockquote":
		out.Kind = "quote"
	case "ul":
		out.Kind = "unorderedList"
	case "ol":
		out.Kind, out.Start = "orderedList", n.listStart(source)
	case "li":
		out.Kind = "listItem"
	case "table":
		out.Kind = "table"
	case "thead":
		out.Kind = "tableHead"
	case "tbody":
		out.Kind = "tableBody"
	case "tfoot":
		out.Kind = "tableFoot"
	case "tr":
		out.Kind = "row"
	case "td", "th":
		out.Kind = "cell"
		if source.Name.Local == "th" {
			out.Kind = "headerCell"
		}
		out.ColSpan, out.RowSpan = n.integerAttribute(source, "colspan"), n.integerAttribute(source, "rowspan")
	case "caption":
		out.Kind = "caption"
	case "em", "i":
		out.Kind = "emphasis"
	case "strong", "b":
		out.Kind = "strong"
	case "s", "del":
		out.Kind = "strike"
	case "sub":
		out.Kind = "subscript"
	case "sup":
		out.Kind = "superscript"
	case "code", "kbd", "samp":
		out.Kind = "code"
	case "pre":
		out.Kind, pre = "preformatted", true
	case "br":
		out.Kind = "break"
	case "hr":
		out.Kind = "separator"
	case "ruby":
		out.Kind = "ruby"
	case "rt":
		out.Kind = "rubyText"
	case "rp":
		out.Kind = "rubyFallback"
	case "img":
		n.image(source, &out, source.attr("", "src"))
	default:
		n.warn("unsupported_content")
	}
	if source.Name.Local == "a" {
		n.link(source, &out)
	}
	for _, role := range strings.Fields(source.attr(epubNamespace, "type")) {
		if role == "footnote" || role == "endnote" || role == "noteref" {
			out.Role = role
			break
		}
	}
	n.anchors(source, &out)
	for i := range source.Children {
		child, err := n.node(&source.Children[i], pre)
		if err != nil {
			return nil, err
		}
		if child != nil {
			out.Children = append(out.Children, *child)
		}
	}
	return &out, nil
}

func (n *sectionNormalizer) anchors(source *xmlElement, out *Node) {
	ids := []string{source.attr("", "id"), source.attr("http://www.w3.org/XML/1998/namespace", "id")}
	if source.is(xhtmlNamespace, "a") {
		ids = append(ids, source.attr("", "name"))
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if out.ID == "" {
			n.anchorCount++
			out.ID = "a" + strconv.Itoa(n.anchorCount)
		}
		if previous, exists := n.section.Anchors[id]; exists && previous != out.ID {
			n.section.Anchors[id] = ""
			n.warn("ambiguous_anchor")
		} else {
			n.section.Anchors[id] = out.ID
		}
	}
}

func elementLanguage(node *xmlElement) string {
	if language := node.attr("http://www.w3.org/XML/1998/namespace", "lang"); language != "" {
		return language
	}
	return node.attr("", "lang")
}

func (n *sectionNormalizer) direction(value string) string {
	if value == "" || value == "ltr" || value == "rtl" || value == "auto" {
		return value
	}
	n.warn("invalid_structure")
	return ""
}

// HTML ordered-list start is a signed 32-bit integer; zero is meaningful.
func (n *sectionNormalizer) listStart(source *xmlElement) *int {
	value := source.attr("", "start")
	if value == "" {
		return nil
	}
	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		n.warn("invalid_structure")
		return nil
	}
	start := int(number)
	return &start
}

func (n *sectionNormalizer) integerAttribute(source *xmlElement, name string) int {
	value := source.attr("", name)
	if value == "" {
		return 0
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 || number > maxCellSpan {
		n.warn("invalid_structure")
		return 0
	}
	return number
}

func hasReadableContent(node Node) bool {
	if node.Kind == "text" && strings.TrimSpace(node.Text) != "" || node.Image != "" {
		return true
	}
	for _, child := range node.Children {
		if hasReadableContent(child) {
			return true
		}
	}
	return false
}

func collapseTextSpace(text string) string {
	var out strings.Builder
	space := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			if !space {
				out.WriteByte(' ')
			}
			space = true
		} else {
			out.WriteRune(r)
			space = false
		}
	}
	return out.String()
}
