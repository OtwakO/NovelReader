package epub

import (
	"encoding/xml"
	"strings"
)

const (
	xhtmlNamespace = "http://www.w3.org/1999/xhtml"
	epubNamespace  = "http://www.idpf.org/2007/ops"
)

// This XML tree is built only through the bounded token decoder. Mixed text
// is accumulated in document order so inline markup cannot reorder labels.
type xmlElement struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Text     string
	Children []xmlElement
}

func (n *xmlElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.Name, n.Attrs = start.Name, start.Attr
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			var child xmlElement
			if err := d.DecodeElement(&child, &token); err != nil {
				return err
			}
			n.Children = append(n.Children, child)
		case xml.CharData:
			n.Children = append(n.Children, xmlElement{Text: string(token)})
		case xml.EndElement:
			return nil
		}
	}
}

func (n *xmlElement) labelText() string {
	var text strings.Builder
	var visit func(*xmlElement)
	visit = func(node *xmlElement) {
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

func (n *xmlElement) is(ns, name string) bool { return n.Name.Space == ns && n.Name.Local == name }
func (n *xmlElement) attr(ns, name string) string {
	for _, a := range n.Attrs {
		if a.Name.Space == ns && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// Ambiguous singleton children are unusable, never silently choose a target.
func (n *xmlElement) child(ns, name string) *xmlElement {
	var found *xmlElement
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
func (n *xmlElement) hasBaseOverride() bool {
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
