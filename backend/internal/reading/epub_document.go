package reading

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/epub"
)

// epubContent projects one successfully prepared, revision-coherent section.
// Storage authorizes publication identity; the provider supplies an application-
// owned, revision-qualified resource issuer. This projection never parses archives.
// Only opaque section-local image keys reach that issuer, never archive paths.
func epubContent(ctx context.Context, revision int64, title string, section epub.PreparedSection, imageHref func(string) string) (Content, error) {
	var project func(epub.Node) (Block, error)
	project = func(source epub.Node) (Block, error) {
		if err := ctx.Err(); err != nil {
			return Block{}, err
		}
		out := Block{Kind: source.Kind, ID: source.ID, Language: source.Language, Direction: source.Direction, Role: source.Role}
		switch source.Kind {
		case "group", "inlineGroup", "paragraph", "quote", "figure", "figureCaption",
			"unorderedList", "listItem", "table", "tableHead", "tableBody", "tableFoot", "row", "caption",
			"emphasis", "strong", "strike", "subscript", "superscript", "code", "preformatted",
			"break", "separator", "ruby", "rubyText", "rubyFallback":
		case "unsupported":
			// Preparation retained safe fallback text and its anchors. The client
			// receives that text, not a provider-specific unsupported-content node.
			out.Kind = "inlineGroup"
		case "text":
			out.Text = source.Text
		case "heading":
			out.Level = source.Level
		case "orderedList":
			if source.Start != nil {
				start := *source.Start
				out.Start = &start
			}
		case "cell", "headerCell":
			out.ColSpan, out.RowSpan = source.ColSpan, source.RowSpan
		case "link":
			out.URL, out.Unavailable = source.URL, source.Unavailable
			if source.Target != nil {
				out.Target = &ReadingTarget{ChapterIndex: source.Target.Section, ContentRevision: revision, Anchor: source.Target.Anchor}
			}
		case "image":
			image, exists := section.Images[source.Image]
			if !exists {
				return Block{}, errors.New("reading: missing prepared image binding")
			}
			out.Resource = &ResourceReference{Href: imageHref(source.Image), MediaType: image.Info.MediaType}
			out.Alt, out.Width, out.Height = source.Alt, image.Info.Width, image.Info.Height
		default:
			return Block{}, errors.New("reading: unsupported prepared prose kind")
		}
		if len(source.Children) != 0 {
			out.Children = make([]Block, len(source.Children))
			for i, child := range source.Children {
				var err error
				out.Children[i], err = project(child)
				if err != nil {
					return Block{}, err
				}
			}
		}
		return out, nil
	}
	root, err := project(section.Root)
	if err != nil {
		return Content{}, err
	}
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	return Content{ContentRevision: revision, Version: StructuredDocumentVersion, Document: Document{Kind: "prose", Title: title, Blocks: []Block{root}, CoverPlaceholder: section.CoverPlaceholder}}, nil
}
