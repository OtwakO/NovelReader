package epub

import (
	"net/url"
	"strconv"
	"strings"
)

func (n *sectionNormalizer) link(source *xmlElement, out *Node) {
	var href string
	found := false
	for _, attr := range source.Attrs {
		if attr.Name.Space == "" && attr.Name.Local == "href" {
			href, found = attr.Value, true
		}
	}
	if !found {
		return
	} // An anchor need not also be a link.
	if external, ok := externalLink(href); ok {
		out.Kind, out.URL = "link", external
		return
	}
	ref, err := resolveReference(n.base, href)
	if err != nil {
		n.warn("link_unavailable")
		return
	}
	key := "l" + strconv.Itoa(len(n.section.Links)+1)
	n.section.Links[key] = ref
	out.Kind, out.Link = "link", key
}

func (n *sectionNormalizer) image(source *xmlElement, out *Node, href string) {
	out.Alt = source.attr("", "alt")
	ref, err := resolveReference(n.base, href)
	if href == "" || err != nil || ref.Fragment != "" {
		out.Kind = "unsupported"
		if out.Alt != "" {
			out.Children = []Node{{Kind: "text", Text: out.Alt}}
		}
		n.warn("image_unavailable")
		return
	}
	key := "r" + strconv.Itoa(len(n.section.Images)+1)
	n.section.Images[key] = ref
	out.Kind, out.Image = "image", key
}

func (n *sectionNormalizer) foreignNode(source *xmlElement) (*Node, error) {
	// A common cover wrapper has exactly one SVG image plus optional metadata.
	// Shapes, groups, filters and other vector behavior are deliberately not
	// approximated. The raster bytes still require independent validation.
	if source.is("http://www.w3.org/2000/svg", "svg") {
		var image *xmlElement
		wrapper := true
		for i := range source.Children {
			child := &source.Children[i]
			switch {
			case child.Name.Local == "" && strings.TrimSpace(child.Text) == "":
			case child.is(source.Name.Space, "title"), child.is(source.Name.Space, "desc"):
			case child.is(source.Name.Space, "image") && image == nil && len(child.Children) == 0:
				image = child
			default:
				wrapper = false
			}
		}
		if wrapper && image != nil {
			href := image.attr("", "href")
			if href == "" {
				href = image.attr("http://www.w3.org/1999/xlink", "href")
			}
			out := Node{}
			n.image(image, &out, href)
			if title := source.child(source.Name.Space, "title"); title != nil {
				out.Alt = strings.Join(strings.Fields(title.labelText()), " ")
			}
			if out.Kind == "unsupported" && out.Alt != "" {
				out.Children = []Node{{Kind: "text", Text: out.Alt}}
			}
			n.anchors(image, &out)
			n.anchors(source, &out)
			return &out, nil
		}
	}
	n.warn("unsupported_content")
	out := Node{Kind: "unsupported"}
	// Preserve meaningful fallback text, never serialize foreign markup.
	var text strings.Builder
	var visit func(*xmlElement)
	visit = func(node *xmlElement) {
		if node.Name.Local == "script" || node.Name.Local == "style" {
			return
		}
		text.WriteString(node.Text)
		for i := range node.Children {
			visit(&node.Children[i])
		}
	}
	visit(source)
	if value := strings.TrimSpace(text.String()); value != "" {
		out.Children = []Node{{Kind: "text", Text: value}}
	}
	n.anchors(source, &out)
	return &out, n.ctx.Err()
}

// Shared by normalization and portable validation; never used to fetch content.
func externalLink(href string) (string, bool) {
	parsed, err := url.Parse(href)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", false
	}
	return parsed.String(), true
}
