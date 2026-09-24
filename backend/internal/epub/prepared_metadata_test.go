package epub

import (
	"context"
	"errors"
	"testing"
)

func summaryFixture() Preparation {
	return Preparation{
		Sections:        []PreparedSectionInfo{{Main: true}, {}},
		ImageProcessing: ImageProcessing{Mode: OriginalImages},
		Navigation: ResolvedNavigation{
			Source: "nav",
			Entries: []ResolvedNavigationEntry{{Children: []ResolvedNavigationEntry{
				{Target: &SectionTarget{Section: 1, Anchor: "n1"}},
				{Unavailable: true},
			}}},
		},
	}
}

func TestValidatePreparedMetadata(t *testing.T) {
	// Empty titles/group labels, auxiliary targets and unavailable entries are
	// legitimate preparation output; this boundary must not infer spine order.
	p := summaryFixture()
	if err := ValidatePreparedMetadata(t.Context(), p, OriginalImages, 2); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Preparation)
	}{
		{"section count", func(p *Preparation) { p.Sections = append(p.Sections, PreparedSectionInfo{}) }},
		{"no main section", func(p *Preparation) { p.Sections[0].Main = false }},
		{"only placeholder main", func(p *Preparation) { p.Sections[0].CoverPlaceholder = true }},
		{"missing section target", func(p *Preparation) { p.Navigation.Entries[0].Children[0].Target.Section = 2 }},
		{"unavailable target", func(p *Preparation) { p.Navigation.Entries[0].Children[0].Unavailable = true }},
		{"spine claims hierarchy", func(p *Preparation) { p.Navigation.Source = "spine" }},
		{"wrong mode", func(p *Preparation) { p.ImageProcessing.Mode = OptimizedImages }},
		{"original claims encoding", func(p *Preparation) { p.ImageProcessing.Backend = "native" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := summaryFixture()
			tc.change(&p)
			if err := ValidatePreparedMetadata(t.Context(), p, OriginalImages, 2); err == nil {
				t.Fatal("invalid summary accepted")
			}
		})
	}
	p = summaryFixture()
	p.ImageProcessing = ImageProcessing{Mode: OptimizedImages, Profile: optimizedImageProfile}
	if err := ValidatePreparedMetadata(t.Context(), p, OptimizedImages, 2); err != nil {
		t.Fatal("image-free optimized book", err)
	}
	p.ImageProcessing.DerivativeCount = 1
	p.ImageProcessing.DerivativeBytes = 10
	if err := ValidatePreparedMetadata(t.Context(), p, OptimizedImages, 2); !errors.Is(err, ErrImagePolicy) {
		t.Fatal("missing backend", err)
	}
	p.ImageProcessing.Backend = "portable"
	if err := ValidatePreparedMetadata(t.Context(), p, OptimizedImages, 2); err != nil {
		t.Fatal("portable backend", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ValidatePreparedMetadata(ctx, p, OptimizedImages, 2); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPreparedNavigationLimits(t *testing.T) {
	p := summaryFixture()
	p.Navigation.Entries = make([]ResolvedNavigationEntry, maxNavigationEntries+1)
	if err := ValidatePreparedMetadata(t.Context(), p, OriginalImages, 2); !errors.Is(err, ErrLimit) {
		t.Fatal("navigation count", err)
	}
	p.Navigation.Entries = nil
	for i := 0; i <= maxXMLDepth; i++ {
		p.Navigation.Entries = []ResolvedNavigationEntry{{Children: p.Navigation.Entries}}
	}
	if err := ValidatePreparedMetadata(t.Context(), p, OriginalImages, 2); !errors.Is(err, ErrLimit) {
		t.Fatal("navigation depth", err)
	}
}
