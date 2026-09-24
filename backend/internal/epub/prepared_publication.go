package epub

import (
	"context"
	"fmt"
)

// PreparedPublicationCheck consumes sections in ordinal order, retaining bounded
// target sets rather than trees. Discard it after any error. Storage still owns
// image-registry and physical-file checks.
type PreparedPublicationCheck struct {
	placeholders []bool
	anchors      map[SectionTarget]struct{}
	references   map[SectionTarget]struct{}
	targetBytes  int
}

func NewPreparedPublicationCheck() *PreparedPublicationCheck {
	return &PreparedPublicationCheck{anchors: map[SectionTarget]struct{}{}, references: map[SectionTarget]struct{}{}}
}

func (p *PreparedPublicationCheck) addTarget(set map[SectionTarget]struct{}, target SectionTarget) error {
	if target.Section < 0 || target.Section >= maxTargetSections {
		return fmt.Errorf("%w: target section missing", ErrPreparedSection)
	}
	if _, exists := set[target]; exists {
		return nil
	}
	// Every valid unique reference is a section start or one of its anchors.
	// Reuse preparation's existing target budgets, including a bounded byte total.
	if len(set) >= maxTargetSections+maxTargetAnchors || len(target.Anchor)+8 > maxTargetBytes-p.targetBytes {
		return ErrLimit
	}
	set[target] = struct{}{}
	p.targetBytes += len(target.Anchor) + 8
	return nil
}

func (p *PreparedPublicationCheck) Finish(ctx context.Context, metadata Preparation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidatePreparedMetadata(ctx, metadata, metadata.ImageProcessing.Mode, len(p.placeholders)); err != nil {
		return err
	}
	for i, placeholder := range p.placeholders {
		if placeholder != metadata.Sections[i].CoverPlaceholder {
			return fmt.Errorf("%w: section summary mismatch", ErrPreparedSection)
		}
	}
	for target := range p.references {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, exists := p.anchors[target]; !exists {
			return fmt.Errorf("%w: link anchor missing", ErrPreparedSection)
		}
	}
	var navigation func([]ResolvedNavigationEntry, int) error
	navigation = func(entries []ResolvedNavigationEntry, depth int) error {
		if len(entries) == 0 {
			return nil
		}
		if depth > maxXMLDepth {
			return ErrLimit
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Target != nil {
				if _, exists := p.anchors[*entry.Target]; !exists {
					return fmt.Errorf("%w: navigation target missing", ErrPreparedSection)
				}
			}
			if err := navigation(entry.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return navigation(metadata.Navigation.Entries, 1)
}
