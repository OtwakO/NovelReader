# BookSource test data

BookSource test data is organized by **purpose**, then by **operation**.

```text
testdata/booksource/
├── conformance/
│   ├── core/                 deterministic Search/detail/TOC/content and rule fixtures
│   └── explore/              deterministic pinned Explore fixtures
└── audits/
    ├── explore/              dated Explore live-audit evidence and fix reruns
    └── search-bookinfo/      dated Search → Book Info evidence and fix reruns
```

## Placement rules

- Put stable offline inputs used by automated tests under `conformance/<operation>/`.
- Put dated observations from live sources under `audits/<operation>/`.
- Keep audit scripts under `scripts/<operation>-audit/vN/`, not beside evidence.
- Keep imported corpus compilations under the ignored repository-root `test-booksources/` directory; audit evidence records their path and SHA-256.
- Add a deterministic test when changing a conformance fixture. Live evidence is historical and must not become a test dependency.
- Preserve stable `(rawIndex, bookSourceUrl)` identity in audit evidence; source name alone is not an identity.

See `conformance/README.md` and `audits/README.md` for the contracts of each category.
