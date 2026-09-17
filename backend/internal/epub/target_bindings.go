package epub

import (
	"context"
	"slices"
)

// ResolvedNavigation preserves TOC hierarchy separately from reading order and
// contains no archive paths or publisher IDs. Empty Entries still require the
// orchestrator's explicitly labeled spine-derived section list.
type ResolvedNavigation struct {
	Source      string
	Entries     []ResolvedNavigationEntry
	Diagnostics []string
}

type ResolvedNavigationEntry struct {
	Label       string
	Target      *SectionTarget
	Unavailable bool
	Children    []ResolvedNavigationEntry
}

// resolveSection replaces private link keys with validated local targets. Broken
// links retain their text/semantics but are explicitly unavailable. External URLs
// are untouched: they remain deliberate actions, never preparation fetches.
// Like image binding, this mutates preparation evidence; discard it on error.
func (t *targetIndex) resolveSection(ctx context.Context, ordinal int, section *Section) error {
	var visit func(*Node) error
	visit = func(node *Node) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if node.Link != "" {
			ref, exists := section.Links[node.Link]
			if exists {
				node.Target = t.resolve(ordinal, ref)
			}
			node.Link = ""
			if node.Target == nil {
				node.Unavailable = true
				if !slices.Contains(section.Diagnostics, "link_unavailable") {
					section.Diagnostics = append(section.Diagnostics, "link_unavailable")
				}
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
		return err
	}
	section.Links = nil
	return ctx.Err()
}

func (t *targetIndex) resolveNavigation(ctx context.Context, source Navigation) (ResolvedNavigation, error) {
	out := ResolvedNavigation{Source: source.Source, Diagnostics: slices.Clone(source.Diagnostics)}
	var visit func([]NavigationEntry) ([]ResolvedNavigationEntry, error)
	visit = func(entries []NavigationEntry) ([]ResolvedNavigationEntry, error) {
		var resolved []ResolvedNavigationEntry
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			node := ResolvedNavigationEntry{Label: entry.Label, Unavailable: entry.Unavailable}
			if entry.Target != nil && !entry.Unavailable {
				node.Target = t.resolve(-1, *entry.Target)
				node.Unavailable = node.Target == nil
			}
			if node.Unavailable && !slices.Contains(out.Diagnostics, "navigation_target_unavailable") {
				out.Diagnostics = append(out.Diagnostics, "navigation_target_unavailable")
			}
			var err error
			node.Children, err = visit(entry.Children)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, node)
		}
		return resolved, nil
	}
	var err error
	out.Entries, err = visit(source.Entries)
	if err != nil {
		return ResolvedNavigation{}, err
	}
	return out, ctx.Err()
}
