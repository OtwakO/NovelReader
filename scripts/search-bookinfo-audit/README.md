# Search → Book Info live-audit scripts

These scripts produce deterministic live evidence for the bounded Search → Book Info audit. They are evidence tooling, not application runtime code.

Each version lives in its own directory:

```text
scripts/search-bookinfo-audit/
└── v1/
    ├── freeze.mjs
    ├── run.mjs
    ├── rerun.mjs
    ├── build-evidence.mjs
    └── verify.mjs
```

Run scripts from the repository root. The normal flow is `freeze` → reset/import/start production → `run` → `rerun` → diagnose → `build-evidence` → `verify`.

The audit intentionally stops after Book Info. It does not request TOC or chapter content and does not modify source/parser behavior.
