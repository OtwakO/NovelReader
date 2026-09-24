# Disposable import-preview comparisons

Question: which navigation and reading-area boundaries should TXT and EPUB previews share?

From the repository root:

```sh
python3 -m http.server 4182 --bind 127.0.0.1 --directory frontend
```

Open http://localhost:4182/prototypes/import-preview/?variant=A . Stop with Ctrl+C.

- **A — Compact toolbar:** contents selector and Previous/Next above one framed preview.
- **B — Visible contents:** chapter list beside the preview; a short scrollable list above it on mobile. Title and author stay visible in the header (not collapsible); Previous/Next use labeled left/right arrow icons.
- **C — Reading first:** a Contents toggle opens navigation in place; selecting a chapter closes it.

Use the bottom arrows or keyboard left/right to switch. Choose desktop/mobile width, TXT/EPUB and import/re-analysis. Chapter selection and Previous/Next work with synthetic local content. EPUB starts with a centered placeholder cover and includes a centered illustration in its prose. Re-analysis separates preview navigation from an explicit mock resume choice. Add/discard actions only display a notice; no requests, imports, persistence or reading-state changes occur.

The app's existing tokens/button styles and placeholder artwork are referenced, not copied. The surrounding review context is simplified, not a proposal to redesign the entire import page. Static isolation follows the repository's existing prototype convention and avoids putting temporary controls into live import workflows.

## Selected direction: B

The user wants whole selected TXT chapters, not the current 4 KiB sample, in import and re-analysis where they can reuse the same presentation without merging approval workflows. Center every displayed image in preview and reader for manual evaluation. Share preview appearance/interaction while retaining format-owned loading and authorization. Do not build a generic second reader.

The user selected B, including its refined persistent title/author, arrow navigation and 250px desktop contents column. Production work is tracked in the [unified import-preview plan](../../../docs/plans/2026-09-25-unified-import-preview.md). No production centering change, TXT endpoint extension or shared component has been implemented here. Prototype chapter text is synthetic and repeated to exercise scrolling, not a test of real TXT loading.

Delete this directory to remove the mock. Nothing imports it into production; implement the selected design properly rather than promoting this code verbatim.
