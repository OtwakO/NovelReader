# BookSource live-audit evidence

Each subdirectory owns one production operation:

- `explore/` — Explore catalog/page samples and post-fix reruns.
- `search-bookinfo/` — Search → Book Info samples and post-fix reruns.

Audit evidence is historical observation, not a deterministic test fixture or a default test dependency. Existing machine-readable audits may contain historical raw source material from before the current policy; do not refresh or extend that pattern.

New committed evidence must be sanitized: record the operation, local audit index or neutral identifier, input/output classification, failure evidence, and durable conclusion without embedding complete source objects, source scripts, cookies, credentials, sensitive headers, or private endpoints. Keep the raw corpus, complete source definitions, and unsanitized machine output only under ignored `test-booksources/` or another explicitly private local location.

Targeted post-fix summaries remain in the same operation directory when they are worth preserving. Scripts live separately under `scripts/<operation>-audit/vN/`. A new operation audit must create its own `audits/<operation>/` directory rather than place files at `testdata/booksource/` or mix them with another operation.
