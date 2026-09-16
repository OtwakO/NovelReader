package epub

import (
	"context"
	"encoding/xml"
	"errors"
	"slices"
	"strings"
)

const (
	maxNavigationEntries = 10000
	xhtmlNamespace       = "http://www.w3.org/1999/xhtml"
	ncxNamespace         = "http://www.daisy.org/z3986/2005/ncx/"
	epubNamespace        = "http://www.idpf.org/2007/ops"
)

// Navigation is inspection evidence, not a catalog. Empty Entries require a
// labeled spine-derived list during preparation. Diagnostics contain codes only.
// Fragment existence and supported content still require section preparation.
type Navigation struct {
	Source      string // "nav", "ncx", or empty when no declaration exists
	Entries     []NavigationEntry
	Diagnostics []string
}

type NavigationEntry struct {
	Label       string
	Target      *Reference // nil for a grouping heading or an unavailable target
	Unavailable bool
	Children    []NavigationEntry
}

func (n *Navigation) warn(code string) {
	if !slices.Contains(n.Diagnostics, code) {
		n.Diagnostics = append(n.Diagnostics, code)
	}
}

func inspectNavigation(ctx context.Context, a *archive, p Package) (Navigation, error) {
	if err := ctx.Err(); err != nil {
		return Navigation{}, err
	}
	var n Navigation
	var selected *Item
	for i := range p.Items {
		item := &p.Items[i]
		if p.Version == "3.0" && slices.Contains(item.Properties, "nav") {
			if selected != nil {
				n.warn("navigation_invalid")
				return n, ctx.Err()
			}
			selected = item
		}
	}
	if selected != nil {
		n.Source = "nav"
	} else if p.Version == "2.0" && p.NCXID != "" {
		n.Source = "ncx"
		for i := range p.Items {
			if p.Items[i].ID == p.NCXID {
				selected = &p.Items[i]
				break
			}
		}
	}
	if selected == nil {
		n.warn("navigation_missing")
		return n, ctx.Err()
	}
	expected := "application/xhtml+xml"
	if n.Source == "ncx" {
		expected = "application/x-dtbncx+xml"
	}
	if selected.MediaType != expected {
		n.warn("navigation_invalid")
		return n, ctx.Err()
	}
	if a.files[selected.Reference.Path] == nil {
		n.warn("navigation_missing")
		return n, ctx.Err()
	}
	data, err := a.readMetadata(ctx, selected.Reference.Path)
	if err != nil {
		return Navigation{}, err
	}
	var root navigationXML
	if err = decodeXML(ctx, data, &root); err != nil {
		if errors.Is(err, ErrLimit) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Navigation{}, err
		}
		n.warn("navigation_invalid")
		return n, ctx.Err()
	}
	// Base overrides are not interpreted: ignoring them would silently redirect
	// targets. Reject this optional navigation instead of guessing or fetching.
	if root.hasBaseOverride() {
		n.warn("navigation_invalid")
		return n, ctx.Err()
	}
	var container *navigationXML
	if n.Source == "ncx" && root.is(ncxNamespace, "ncx") {
		container = root.child(ncxNamespace, "navMap")
	} else if n.Source == "nav" && root.is(xhtmlNamespace, "html") {
		var navs []*navigationXML
		root.tocNodes(&navs)
		if len(navs) == 1 {
			container = navs[0].child(xhtmlNamespace, "ol")
		}
	}
	if container == nil {
		n.warn("navigation_invalid")
		return n, ctx.Err()
	}
	paths := make(map[string]bool, len(p.Items))
	for _, item := range p.Items {
		paths[item.Reference.Path] = true
	}
	count := 0
	n.Entries, err = n.parseEntries(ctx, container, selected.Reference.Path, a, paths, &count)
	if err != nil {
		return Navigation{}, err
	}
	if len(n.Entries) == 0 {
		n.warn("navigation_empty")
	}
	return n, ctx.Err()
}

func (n *Navigation) parseEntries(ctx context.Context, container *navigationXML, base string, a *archive, paths map[string]bool, count *int) ([]NavigationEntry, error) {
	var entries []NavigationEntry
	for i := range container.Children {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := &container.Children[i]
		ns, name := xhtmlNamespace, "li"
		if n.Source == "ncx" {
			ns, name = ncxNamespace, "navPoint"
		}
		if node.Name.Local == "" {
			continue
		}
		if !node.is(ns, name) {
			n.warn("navigation_invalid")
			continue
		}
		(*count)++
		if *count > maxNavigationEntries {
			return nil, ErrLimit
		}
		var entry NavigationEntry
		var label, children *navigationXML
		href := ""
		link := false
		if n.Source == "ncx" {
			if wrapper := node.child(ns, "navLabel"); wrapper != nil {
				label = wrapper.child(ns, "text")
			}
			if content := node.child(ns, "content"); content != nil {
				href = content.attr("", "src")
			}
			link, children = true, node
		} else {
			label = node.child(ns, "a")
			if label != nil {
				link, href = true, label.attr("", "href")
			} else {
				label = node.child(ns, "span")
			}
			children = node.child(ns, "ol")
		}
		if label != nil {
			entry.Label = strings.Join(strings.Fields(label.labelText()), " ")
		}
		if entry.Label == "" {
			n.warn("navigation_label_missing")
		}
		if link {
			ref, err := resolveReference(base, href)
			if href == "" || err != nil || !paths[ref.Path] || a.files[ref.Path] == nil {
				entry.Unavailable = true
				n.warn("navigation_target_unavailable")
			} else {
				entry.Target = &ref
			}
		} else if label == nil || children == nil {
			n.warn("navigation_invalid")
		}
		if children != nil {
			// NCX contains label/content alongside its nested navPoints.
			if n.Source == "ncx" {
				filtered := navigationXML{}
				for _, child := range children.Children {
					if child.is(ncxNamespace, "navPoint") {
						filtered.Children = append(filtered.Children, child)
					}
				}
				children = &filtered
			}
			var err error
			entry.Children, err = n.parseEntries(ctx, children, base, a, paths, count)
			if err != nil {
				return nil, err
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// This small XML tree is used only for bounded navigation metadata. Mixed text
// is accumulated in document order so inline markup cannot reorder labels.
type navigationXML struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Text     string
	Children []navigationXML
}

func (n *navigationXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.Name, n.Attrs = start.Name, start.Attr
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			var child navigationXML
			if err := d.DecodeElement(&child, &token); err != nil {
				return err
			}
			n.Children = append(n.Children, child)
		case xml.CharData:
			n.Children = append(n.Children, navigationXML{Text: string(token)})
		case xml.EndElement:
			return nil
		}
	}
}

func (n *navigationXML) labelText() string {
	var text strings.Builder
	var visit func(*navigationXML)
	visit = func(node *navigationXML) {
		if node.is(xhtmlNamespace, "br") {
			text.WriteByte(' ')
			return
		}
		text.WriteString(node.Text)
		if len(node.Children) == 0 {
			alternate := node.attr("", "alt")
			if alternate == "" {
				alternate = node.attr("", "title")
			}
			text.WriteString(alternate)
		}
		for i := range node.Children {
			visit(&node.Children[i])
		}
	}
	visit(n)
	return text.String()
}

func (n *navigationXML) is(ns, name string) bool { return n.Name.Space == ns && n.Name.Local == name }
func (n *navigationXML) attr(ns, name string) string {
	for _, a := range n.Attrs {
		if a.Name.Space == ns && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// Ambiguous singleton children are unusable, never silently choose a target.
func (n *navigationXML) child(ns, name string) *navigationXML {
	var found *navigationXML
	for i := range n.Children {
		if n.Children[i].is(ns, name) {
			if found != nil {
				return nil
			}
			found = &n.Children[i]
		}
	}
	return found
}
func (n *navigationXML) tocNodes(nodes *[]*navigationXML) {
	if n.is(xhtmlNamespace, "nav") && slices.Contains(strings.Fields(n.attr(epubNamespace, "type")), "toc") {
		*nodes = append(*nodes, n)
	}
	for i := range n.Children {
		n.Children[i].tocNodes(nodes)
	}
}
func (n *navigationXML) hasBaseOverride() bool {
	if n.is(xhtmlNamespace, "base") || n.attr("http://www.w3.org/XML/1998/namespace", "base") != "" {
		return true
	}
	for i := range n.Children {
		if n.Children[i].hasBaseOverride() {
			return true
		}
	}
	return false
}
