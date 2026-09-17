package epub

import (
	"context"
	"errors"
	"slices"
)

// One resolver belongs to one sequential preparation. Repeated illustrations
// share validation metadata, not decoded bitmaps or byte caches. Its key set is
// bounded by the inspected manifest; cancellation/I/O/limit failures aren't cached.
type imageResolver struct {
	archive *archive
	items   map[string]Item
	cache   map[string]imageResult
}

type imageResult struct {
	info ImageInfo
	err  error
}

func newImageResolver(a *archive, items []Item) *imageResolver {
	r := &imageResolver{archive: a, items: make(map[string]Item, len(items)), cache: map[string]imageResult{}}
	for _, item := range items {
		r.items[item.Reference.Path] = item
	}
	return r
}

func optionalImageError(err error) bool {
	return errors.Is(err, ErrImageUnavailable) || errors.Is(err, ErrImageUnsupported) || errors.Is(err, ErrImageInvalid)
}

func (r *imageResolver) resolve(ctx context.Context, ref Reference) (ImageInfo, error) {
	if err := ctx.Err(); err != nil {
		return ImageInfo{}, err
	}
	if ref.Fragment != "" {
		return ImageInfo{}, ErrImageUnavailable
	}
	item, found := r.items[ref.Path]
	if !found {
		return ImageInfo{}, ErrImageUnavailable
	}
	if result, found := r.cache[ref.Path]; found {
		return result.info, result.err
	}
	var info ImageInfo
	var err error
	switch {
	case item.MediaType != "image/jpeg" && item.MediaType != "image/png":
		err = ErrImageUnsupported
	case r.archive.files[ref.Path] == nil:
		err = ErrImageUnavailable
	default:
		var data []byte
		data, err = r.archive.readBounded(ctx, ref.Path, maxRasterBytes, int64(maxExpandedBytes))
		if err == nil {
			info, err = ValidateImage(ctx, data, item.MediaType)
		}
	}
	if err == nil || optionalImageError(err) {
		r.cache[ref.Path] = imageResult{info: info, err: err}
	}
	return info, err
}

// resolveSection mutates only preparation evidence; discard it on error. Missing
// optional images retain an explicit placeholder/alt text and a review diagnostic.
// The returned readable flag is evidence for whole-book admission (a failed cover
// must not automatically reject otherwise readable prose), not publication approval.
func (r *imageResolver) resolveSection(ctx context.Context, section *Section) (bool, error) {
	var visit func(*Node) error
	visit = func(node *Node) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if node.Image != "" {
			ref, exists := section.Images[node.Image]
			var info ImageInfo
			err := ErrImageUnavailable
			if exists {
				info, err = r.resolve(ctx, ref)
			}
			if err != nil {
				if !optionalImageError(err) {
					return err
				}
				code := "image_unavailable"
				if errors.Is(err, ErrImageUnsupported) {
					code = "image_unsupported"
				}
				if errors.Is(err, ErrImageInvalid) {
					code = "image_invalid"
				}
				if !slices.Contains(section.Diagnostics, code) {
					section.Diagnostics = append(section.Diagnostics, code)
				}
				delete(section.Images, node.Image)
				node.Kind, node.Image, node.Width, node.Height = "unsupported", "", 0, 0
				node.Children = nil
				if node.Alt != "" {
					node.Children = []Node{{Kind: "text", Text: node.Alt}}
				}
			} else {
				node.Width, node.Height = info.Width, info.Height
			}
		}
		for i := range node.Children {
			if err := visit(&node.Children[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(&section.Root); err != nil {
		return false, err
	}
	readable := hasReadableContent(section.Root)
	if !readable && !slices.Contains(section.Diagnostics, "no_readable_content") {
		section.Diagnostics = append(section.Diagnostics, "no_readable_content")
	}
	return readable, ctx.Err()
}
