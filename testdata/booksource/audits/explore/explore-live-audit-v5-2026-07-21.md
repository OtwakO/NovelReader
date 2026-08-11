# Explore Live Compatibility Audit — Final Targeted Batch

## Scope

- **Date:** 2026-07-21
- **Corpus:** `test_booksource4.json`
- **SHA-256:** `23d4db8fda293020843645ed3f29fb49236f5e1bff2f38286eac16caab54598c`
- **Seed:** `NovelReader-explore-final-targeted-2026-07-21`
- **Sample:** 25 unique enabled Explore URLs, disjoint from all 200 identities in batches 1–4.
- **Composition:** 15 sources selected from affected rule families and 10 ordinary controls.
- **Execution:** fresh isolated database, first selectable category/page 1, four clients; every non-pass rerun sequentially; browser/live-body checks only for credible engine candidates.

## Result

| Classification | Count |
|---|---:|
| Credible non-empty | 17 |
| Shared engine gaps | 2 |
| Upstream HTTP/DNS | 2 |
| Stale source URL contract | 1 |
| Incomplete source/site drift | 1 |
| Site markup drift | 1 |
| Invalid source script | 1 |
| **Total** | **25** |

The 17 credible sources returned **355 books**, all with distinct URLs within each source. Ten of the 15 affected-family sources and seven of the 10 controls passed.

## Shared gaps

### Raw 84 — `tbody@tr` bare-child ambiguity

The live page contains 20 `tbody tr` book rows. The production Analyzer returns one `<tbody>` for `tbody@tr`, while `tag.tbody@tag.tr` and CSS `tbody tr` return all 20 rows. Full Explore parsing consequently returns only the first book. This is shared Default-routing behavior: a bare child token after `@` is still interpreted as an attribute getter in this unpositioned form.

### Raw 565 — missing `java.t2s`

The valid generated-catalog script fails with `TypeError: Object has no member 't2s'`. Replacing `java.t2s(t)` with identity `t` lets the same production runtime generate and parse **470 categories**. Legado documents `java.t2s(text)`, and nine corpus sources call it, so this is a focused shared Java-bridge gap.

## Non-engine outcomes

- Raw 756: upstream HTTP 521.
- Raw 275: DNS resolution failure.
- Raw 710: live redirect from `qudushu.com` to `qudushu.la`; 15 rows parse when the imported domain-specific `bookUrlPattern` is removed.
- Raw 455: empty Explore result rules and redirect to the generic iQIYI home page.
- Raw 932: live replacement domain, but imported selectors match none of the new markup.
- Raw 208: malformed source JavaScript uses `.map([title,id]=>...)` rather than a destructured parameter.

## Recommendation

Stop broad random auditing after these two focused compatibility gaps are resolved and regression-tested. Their fixes should be followed by targeted corpus reruns for `tbody@bareChild` and `java.t2s`, not another unrestricted sample.

Machine-readable evidence: `testdata/booksource/audits/explore/explore-live-audit-v5-2026-07-21.json`.
