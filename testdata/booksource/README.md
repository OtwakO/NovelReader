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

## BookSource data policy

Complete or real-world BookSource definitions are private local inputs. Do not commit or push new real BookSources, raw source objects, corpus extracts, or audit output that embeds their rules, scripts, headers, cookies, credentials, private endpoints, or complete source JSON. This applies even when the repository or remote is private.

Keep imported corpora and complete sources under the ignored repository-root `test-booksources/` directory. Before committing any BookSource-related fixture or report, inspect its content rather than relying on its filename or directory: machine-generated audit JSON can embed the full source object.

Repository fixtures must be synthetic and minimal. Invent only the fields, rules, responses, and URLs needed to prove a NovelReader contract; use reserved domains such as `example.com` and local test servers. Do not copy and lightly redact a real source when a small purpose-built fixture can express the behavior.

Historical tracked fixtures and audit evidence predate this policy. Their presence is not precedent for adding more. Do not expand or refresh raw historical source data; if related work is needed, keep the real input local and commit only a sanitized conclusion or a minimal synthetic regression.

## Test policy

- Required GitHub Actions tests must be deterministic and pass from a clean checkout without `test-booksources/`, live websites, credentials, cookies, or developer-local state.
- Tests whose value depends on a complete real BookSource are optional local compatibility checks. They must skip clearly when their ignored private fixture is absent; absence must not fail the default suite.
- Do not create a duplicate synthetic test merely to mirror every private compatibility test.
- Add or extend a deterministic synthetic regression only when a real source reveals a reusable NovelReader defect that is not already covered. Prefer extending an existing focused test when that remains clear.
- Live audits are explicit local workflows, not dependencies of `go test ./...` or container publication.

## Placement rules

- Put stable synthetic offline inputs used by automated tests under `conformance/<operation>/`.
- Put only sanitized dated observations from live sources under `audits/<operation>/`; omit embedded source definitions and sensitive request state.
- Keep audit scripts under `scripts/<operation>-audit/vN/`, not beside evidence.
- Keep imported corpus compilations and complete source definitions under ignored `test-booksources/`.
- A deterministic fixture change must remain covered by a focused test. Live evidence is historical and must not become a default test dependency.
- When identity is needed in sanitized audit evidence, prefer a local audit index and hash; include a real source URL only when it is necessary, non-sensitive, and cannot be replaced by a neutral identifier.

See `conformance/README.md` and `audits/README.md` for the contracts of each category.
