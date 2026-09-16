package epub

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrFixedLayout = errors.New("epub: fixed layout is unsupported")
	ErrEncrypted   = errors.New("epub: encrypted resources are unsupported")
)

const renditionVocabulary = "http://www.idpf.org/vocab/rendition/#"

// classifySupport verifies package declarations, not chapter semantics. No
// successful result here authorizes publication before content preparation.
func classifySupport(ctx context.Context, a *archive, p *Package, doc packageXML) error {
	prefixes := map[string]string{"rendition": renditionVocabulary}
	fields := strings.Fields(doc.Prefix)
	if len(fields)%2 != 0 {
		return ErrPackage
	}
	for i := 0; i < len(fields); i += 2 {
		if !strings.HasSuffix(fields[i], ":") {
			return ErrPackage
		}
		prefix := strings.TrimSuffix(fields[i], ":")
		if prefix == "rendition" && fields[i+1] != renditionVocabulary {
			return ErrPackage
		}
		prefixes[prefix] = fields[i+1]
	}
	expand := func(term string) string {
		prefix, local, found := strings.Cut(term, ":")
		if found && prefixes[prefix] != "" {
			return prefixes[prefix] + local
		}
		return term
	}
	for _, meta := range doc.Metadata.Meta {
		if meta.Refines != "" {
			continue
		}
		if expand(meta.Property) == renditionVocabulary+"layout" {
			switch strings.TrimSpace(meta.Value) {
			case "pre-paginated":
				return fmt.Errorf("%w: %w", ErrUnsupported, ErrFixedLayout)
			case "reflowable":
			default:
				return ErrPackage
			}
		}
		// Older EPUB 2 producers use the legacy iBooks declaration.
		if meta.Name == "fixed-layout" && strings.TrimSpace(meta.Content) == "true" {
			return fmt.Errorf("%w: %w", ErrUnsupported, ErrFixedLayout)
		}
	}
	if err := checkLegacyLayout(ctx, a); err != nil {
		return err
	}
	items := make(map[string]Item, len(p.Items))
	paths := make(map[string]Item, len(p.Items))
	for _, item := range p.Items {
		items[item.ID], paths[item.Reference.Path] = item, item
	}
	if err := checkEncryption(ctx, a, paths); err != nil {
		return err
	}
	usedFallback := false
	for i := range p.Spine {
		spine := &p.Spine[i]
		for _, property := range spine.Properties {
			if expand(property) == renditionVocabulary+"layout-pre-paginated" {
				return fmt.Errorf("%w: %w", ErrUnsupported, ErrFixedLayout)
			}
		}
		item, err := resolveSpineContent(ctx, a, items, spine.ID)
		if err != nil {
			return err
		}
		spine.ContentID = item.ID
		usedFallback = usedFallback || item.ID != spine.ID
	}
	if usedFallback {
		p.Diagnostics = append(p.Diagnostics, "spine_fallback_used")
	}
	return ctx.Err()
}

func resolveSpineContent(ctx context.Context, a *archive, items map[string]Item, id string) (Item, error) {
	visited := make(map[string]bool)
	for {
		if err := ctx.Err(); err != nil {
			return Item{}, err
		}
		item, found := items[id]
		if !found || visited[id] {
			return Item{}, ErrPackage
		}
		visited[id] = true
		if item.MediaType == "application/xhtml+xml" {
			if a.files[item.Reference.Path] == nil {
				return Item{}, fmt.Errorf("%w: missing spine content", ErrPackage)
			}
			return item, nil
		}
		if item.Fallback == "" {
			return Item{}, fmt.Errorf("%w: spine media type", ErrUnsupported)
		}
		id = item.Fallback
	}
}

// Legacy EPUB 2 display options can carry the only fixed-layout declaration.
func checkLegacyLayout(ctx context.Context, a *archive) error {
	for _, name := range []string{"META-INF/com.apple.ibooks.display-options.xml", "META-INF/com.kobobooks.display-options.xml"} {
		if a.files[name] == nil {
			continue
		}
		data, err := a.readMetadata(ctx, name)
		if err != nil {
			return err
		}
		var doc struct {
			Options []struct {
				Name  string `xml:"name,attr"`
				Value string `xml:",chardata"`
			} `xml:"platform>option"`
		}
		if err := decodeXML(ctx, data, &doc); err != nil {
			return err
		}
		for _, option := range doc.Options {
			if option.Name == "fixed-layout" && strings.TrimSpace(option.Value) == "true" {
				return fmt.Errorf("%w: %w", ErrUnsupported, ErrFixedLayout)
			}
		}
	}
	return ctx.Err()
}
