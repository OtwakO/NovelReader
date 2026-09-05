# Search → Book Info live-audit scripts

These are explicit local evidence tools, not application runtime code or default CI tests. Keep all source-bearing raw output under ignored `test-booksources/`; only sanitized conclusions belong under `testdata/booksource/audits/search-bookinfo/`.

**Current engine audit:** use [v5](v5/README.md), which freezes the user-selected corpus and calls production Searcher methods without resetting/importing into an installed account. See its README for commands, output ownership and limits. Older versions reconstruct parts of search execution and can emit complete sources into tracked paths; do not run them unchanged under the current data policy.

Historical versions v1–v4 live in their own directories, with layouts such as:

```text
scripts/search-bookinfo-audit/
└── v1/
    ├── freeze.mjs
    ├── run.mjs
    ├── rerun.mjs
    ├── build-evidence.mjs
    └── verify.mjs
```

Run scripts from the repository root. The v5 flow is `freeze` → build local probe → `initial` → sequential `replay`/`scrutiny` → diagnose → write sanitized findings. Do not apply the older reset/import flow to the developer's current data.

The audit intentionally stops after Book Info. New versions follow `.agents/skills/booksource-audit-workflow/` and `.agents/skills/search-bookinfo-live-audit/`.
