import type { CatalogNavigation, CatalogNavigationEntry } from '../../api/catalog-navigation';
import type { ReadingTarget } from '../../api/reading-target';

/** Search keeps ancestor context without reordering or inventing section identities. */
export function searchNavigation(entries: CatalogNavigationEntry[], query: string) {
  const search = query.trim().toLocaleLowerCase();
  let matches = 0;
  function visit(nodes: CatalogNavigationEntry[]): CatalogNavigationEntry[] {
    return nodes.flatMap(node => {
      const matched = !search || node.label.toLocaleLowerCase().includes(search);
      if (matched) matches++;
      const children = visit(node.children);
      return matched || children.length ? [{ ...node, children }] : [];
    });
  }
  return { entries: visit(entries), matches };
}

export function currentNavigationTarget(entries: CatalogNavigationEntry[], index: number, anchor?: string): ReadingTarget | undefined {
  let first: ReadingTarget | undefined;
  let exact: ReadingTarget | undefined;
  function visit(nodes: CatalogNavigationEntry[]) {
    for (const node of nodes) {
      if (node.target?.chapterIndex === index) {
        first ??= node.target;
        if (node.target.anchor === anchor) exact ??= node.target;
      }
      visit(node.children);
    }
  }
  visit(entries);
  return exact ?? first;
}

/** Preorder text mapping preserves canonical targets and authored structure. */
export function mapNavigationLabels(navigation: CatalogNavigation, map: (label: string) => string): CatalogNavigation {
  function visit(nodes: CatalogNavigationEntry[]): CatalogNavigationEntry[] {
    return nodes.map(node => ({ ...node, label: map(node.label), children: visit(node.children) }));
  }
  return { ...navigation, entries: visit(navigation.entries) };
}
