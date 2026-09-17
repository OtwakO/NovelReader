package epub

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	maxManifestItems = 10000
	maxSpineItems    = 5000
)

// Package is an inspected inventory, NOT a prepared or publishable book.
// Content semantics, navigation target anchors and media bytes must still pass
// preparation. These archive references must never be serialized to clients.
type Package struct {
	Version     string
	Path        string
	Title       string
	Authors     []string
	Language    string
	Items       []Item
	Spine       []SpineItem
	NCXID       string
	CoverPages  []Reference
	Navigation  Navigation
	Diagnostics []string
}

type Item struct {
	ID         string
	Reference  Reference
	MediaType  string
	Properties []string
	Fallback   string
}

type SpineItem struct {
	// ContentID selects the supported manifest item after declared fallbacks.
	// ID retains the original spine identity; order and Linear never change.
	ContentID  string
	ID         string
	Linear     bool
	Properties []string
}

// Inspect checks package support declarations and optional navigation. It leaves original ownership
// with the caller, does not retain chapter bytes, and has no network access.
func Inspect(ctx context.Context, original io.ReaderAt, size int64) (Package, error) {
	a, err := openArchive(ctx, original, size)
	if err != nil {
		return Package{}, err
	}
	return inspectArchive(ctx, a)
}

func inspectArchive(ctx context.Context, a *archive) (Package, error) {
	data, err := a.readMetadata(ctx, "META-INF/container.xml")
	if err != nil {
		return Package{}, err
	}
	var container struct {
		XMLName xml.Name `xml:"urn:oasis:names:tc:opendocument:xmlns:container container"`
		Roots   []struct {
			Path      string `xml:"full-path,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := decodeXML(ctx, data, &container); err != nil {
		return Package{}, err
	}
	// Multiple renditions are outside the milestone. Select the first OPF
	// rootfile in container order rather than guessing from ZIP filenames.
	var root string
	for _, candidate := range container.Roots {
		if candidate.MediaType == "application/oebps-package+xml" {
			root = candidate.Path
			break
		}
	}
	// OCF full-path is an archive path, not a URI: do not percent-decode it.
	if !validEntryName(root) {
		return Package{}, ErrPackage
	}
	data, err = a.readMetadata(ctx, root)
	if err != nil {
		return Package{}, err
	}
	var doc packageXML
	if err := decodeXML(ctx, data, &doc); err != nil {
		return Package{}, err
	}
	p, err := inspectPackage(ctx, root, doc)
	if err != nil {
		return Package{}, err
	}
	if err := classifySupport(ctx, a, &p, doc); err != nil {
		return Package{}, err
	}
	p.Navigation, err = inspectNavigation(ctx, a, p)
	if err != nil {
		return Package{}, err
	}
	return p, nil
}

func inspectPackage(ctx context.Context, name string, doc packageXML) (Package, error) {
	if doc.Version != "2.0" && doc.Version != "3.0" {
		return Package{}, ErrUnsupported
	}
	if len(doc.Items) == 0 || len(doc.Spine.Items) == 0 {
		return Package{}, ErrPackage
	}
	if len(doc.Items) > maxManifestItems || len(doc.Spine.Items) > maxSpineItems {
		return Package{}, ErrLimit
	}
	p := Package{Version: doc.Version, Path: name, Title: strings.TrimSpace(doc.Metadata.Title), Language: strings.TrimSpace(doc.Metadata.Language), NCXID: doc.Spine.TOC}
	for _, author := range doc.Metadata.Authors {
		if author = strings.TrimSpace(author); author != "" {
			p.Authors = append(p.Authors, author)
		}
	}
	ids := make(map[string]bool, len(doc.Items))
	paths := make(map[string]string, len(doc.Items))
	for _, item := range doc.Items {
		if err := ctx.Err(); err != nil {
			return Package{}, err
		}
		if item.ID == "" || ids[item.ID] || item.Href == "" || item.MediaType == "" {
			return Package{}, ErrPackage
		}
		ref, err := resolveReference(name, item.Href)
		if err != nil {
			return Package{}, fmt.Errorf("%w: %w", ErrPackage, err)
		}
		// Multiple IDs may alias one resource. Conflicting media declarations
		// cannot safely share resource-level encryption/content classification.
		if ref.Fragment != "" || (paths[ref.Path] != "" && paths[ref.Path] != item.MediaType) {
			return Package{}, ErrPackage
		}
		ids[item.ID] = true
		paths[ref.Path] = item.MediaType
		p.Items = append(p.Items, Item{ID: item.ID, Reference: ref, MediaType: item.MediaType, Properties: strings.Fields(item.Properties), Fallback: item.Fallback})
	}
	linear := 0
	seen := make(map[string]bool, len(doc.Spine.Items))
	for _, item := range doc.Spine.Items {
		if err := ctx.Err(); err != nil {
			return Package{}, err
		}
		if !ids[item.ID] || seen[item.ID] || (item.Linear != "" && item.Linear != "yes" && item.Linear != "no") {
			return Package{}, ErrPackage
		}
		seen[item.ID] = true
		main := item.Linear != "no"
		if main {
			linear++
		}
		p.Spine = append(p.Spine, SpineItem{ID: item.ID, Linear: main, Properties: strings.Fields(item.Properties)})
	}
	if linear == 0 {
		return Package{}, ErrPackage
	}
	if len(doc.Guide.References) > maxNavigationEntries {
		return Package{}, ErrLimit
	}
	for _, ref := range doc.Guide.References {
		if ref.Type != "cover" {
			continue
		}
		if target, err := resolveReference(name, ref.Href); err == nil && target.Fragment == "" && paths[target.Path] == "application/xhtml+xml" {
			p.CoverPages = append(p.CoverPages, target)
		}
	}
	if err := ctx.Err(); err != nil {
		return Package{}, err
	}
	return p, nil
}

type packageXML struct {
	XMLName  xml.Name `xml:"http://www.idpf.org/2007/opf package"`
	Version  string   `xml:"version,attr"`
	Prefix   string   `xml:"prefix,attr"`
	Metadata struct {
		Title    string   `xml:"http://purl.org/dc/elements/1.1/ title"`
		Authors  []string `xml:"http://purl.org/dc/elements/1.1/ creator"`
		Language string   `xml:"http://purl.org/dc/elements/1.1/ language"`
		Meta     []struct {
			Property string `xml:"property,attr"`
			Refines  string `xml:"refines,attr"`
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Value    string `xml:",chardata"`
		} `xml:"http://www.idpf.org/2007/opf meta"`
	} `xml:"http://www.idpf.org/2007/opf metadata"`
	Items []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
		Fallback   string `xml:"fallback,attr"`
	} `xml:"manifest>item"`
	Guide struct {
		References []struct {
			Type string `xml:"type,attr"`
			Href string `xml:"href,attr"`
		} `xml:"http://www.idpf.org/2007/opf reference"`
	} `xml:"http://www.idpf.org/2007/opf guide"`
	Spine struct {
		TOC   string `xml:"toc,attr"`
		Items []struct {
			ID         string `xml:"idref,attr"`
			Linear     string `xml:"linear,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"itemref"`
	} `xml:"http://www.idpf.org/2007/opf spine"`
}
