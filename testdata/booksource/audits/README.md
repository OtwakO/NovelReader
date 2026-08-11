# BookSource live-audit evidence

Each subdirectory owns one production operation:

- `explore/` — Explore catalog/page samples and post-fix reruns.
- `search-bookinfo/` — Search → Book Info samples and post-fix reruns.

Audit evidence is historical observation, not a deterministic test fixture. Each audit stores machine-readable JSON and a concise Markdown report; targeted post-fix reruns remain in the same operation directory. Scripts live separately under `scripts/<operation>-audit/vN/`.

New operation audits must create their own `audits/<operation>/` directory rather than place files at `testdata/booksource/` or mix them with another operation.
