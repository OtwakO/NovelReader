package epub

import (
	"slices"
	"strings"
)

// EPUB 3 declarations take precedence over legacy cover metadata. Resource
// aliases are one choice, but conflicting declarations are never ranked by
// filename, manifest order, image size or which file happens to decode.
func inspectCoverImage(p *Package, doc packageXML) {
	items := make(map[string]Item, len(p.Items))
	candidates := make(map[string]Reference)
	modern := false
	for _, item := range p.Items {
		items[item.ID] = item
		if p.Version == "3.0" && slices.Contains(item.Properties, "cover-image") {
			modern = true
			candidates[item.Reference.Path] = item.Reference
		}
	}
	invalid := make(map[string]bool)
	if !modern {
		for _, meta := range doc.Metadata.Meta {
			if meta.Name != "cover" {
				continue
			}
			id := strings.TrimSpace(meta.Content)
			item, exists := items[id]
			if !exists {
				invalid[id] = true
				continue
			}
			candidates[item.Reference.Path] = item.Reference
		}
	}
	switch {
	case len(candidates)+len(invalid) > 1:
		p.CoverImageAmbiguous = true
		p.Diagnostics = append(p.Diagnostics, "cover_image_ambiguous")
	case len(invalid) != 0:
		p.Diagnostics = append(p.Diagnostics, "cover_declaration_invalid")
	case len(candidates) == 1:
		for _, ref := range candidates {
			p.CoverImage = &ref
		}
	}
}

// Streaming cover-page evidence uses constant space. Observe before validation
// removes broken image bindings: a surviving decorative logo must not become
// the cover merely because another image failed. Repeated references to the
// same resource, including manifest aliases, are not competing artwork.
type coverPageImage struct {
	reference   *Reference
	ambiguous   bool
	unavailable bool
}

func (c *coverPageImage) observe(section Section, items map[string]Item) {
	if slices.Contains(section.Diagnostics, "image_unavailable") || slices.Contains(section.Diagnostics, "unsupported_content") {
		c.unavailable = true
	}
	for _, ref := range section.Images {
		if _, declared := items[ref.Path]; !declared {
			c.unavailable = true
			continue
		}
		if c.reference == nil {
			c.reference = &ref
		} else if *c.reference != ref {
			c.ambiguous = true
		}
	}
}
