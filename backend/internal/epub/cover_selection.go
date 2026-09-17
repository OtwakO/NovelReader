package epub

import (
	"context"
	"maps"
	"slices"
)

func (w *preparationWork) selectCover(ctx context.Context) (*PreparedImage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A weaker source must not arbitrate conflicting authoritative declarations.
	if w.packageInfo.CoverImageAmbiguous {
		return nil, nil
	}
	if ref := w.packageInfo.CoverImage; ref != nil {
		image, err := w.coverResource(ctx, *ref)
		if err != nil || image != nil {
			return image, err
		}
	}
	// Cover pages outside the reading graph are inspected only for artwork. They
	// do not create sections, discover notes or change reading-order membership.
	for _, path := range slices.Sorted(maps.Keys(w.covers)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if w.seen[path] {
			continue
		} // already observed while preparing sections
		if w.archive.files[path] == nil {
			w.coverPageImage.unavailable = true
			continue
		}
		data, err := w.archive.readSection(ctx, path)
		if err != nil {
			return nil, err
		}
		section, err := normalizeSection(ctx, data, path)
		if err != nil {
			if !optionalSectionError(err) {
				return nil, err
			}
			w.coverPageImage.unavailable = true
			continue
		}
		w.coverPageImage.observe(section, w.items)
	}
	choice := w.coverPageImage
	if choice.ambiguous {
		w.result.warn("cover_image_ambiguous")
		return nil, nil
	}
	if choice.unavailable {
		w.result.warn("cover_image_unavailable")
		return nil, nil
	}
	if choice.reference == nil {
		return nil, ctx.Err()
	} // no artwork is not a failure
	return w.coverResource(ctx, *choice.reference)
}

func (w *preparationWork) coverResource(ctx context.Context, ref Reference) (*PreparedImage, error) {
	info, err := w.images.resolve(ctx, ref)
	if err != nil {
		if !optionalImageError(err) {
			return nil, err
		}
		w.result.warn("cover_" + imageDiagnostic(err))
		return nil, nil
	}
	return &PreparedImage{Reference: ref, Info: info}, nil
}
