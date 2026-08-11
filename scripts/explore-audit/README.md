# Explore live-audit scripts

These historical utilities produced and verified deterministic Explore live-audit evidence in
`testdata/booksource/audits/explore/`.

They are grouped by audit version:

```text
scripts/explore-audit/
├── v10/
├── v11/
├── v12/
└── v13/
```

Run every script from the repository root because corpus, evidence, and `/tmp` paths are intentionally
root-relative. For example:

```bash
node scripts/explore-audit/v12/freeze.mjs
node scripts/explore-audit/v12/run.mjs
node scripts/explore-audit/v12/rerun.mjs
node scripts/explore-audit/v12/build-evidence.mjs
node scripts/explore-audit/v12/verify.mjs
```

The normal version flow is `freeze` → `run` → `rerun` → `build-evidence` → `verify`. Versions with
`rerun-fixes.mjs` and `verify-fixes.mjs` also record the targeted post-fix production replay.

These scripts are evidence tooling, not application runtime code. New Explore audits should use the
project audit skills in `.agents/skills/booksource-audit-workflow/` and `.agents/skills/explore-live-audit/`, and keep generated evidence under `testdata/booksource/audits/explore/`.
