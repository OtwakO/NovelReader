---
status: completed
updated: 2026-09-02
---

# Source Collection Availability

## Goal

Let a reader temporarily remove an entire Source Collection from Search and Explore without changing any member BookSource's individual `enabled` or `enabledExplore` preference.

Done means:

- each collection has a persisted availability flag, enabled by default;
- disabling a collection removes its member sources from Search and Explore;
- re-enabling restores the effective availability implied by each source's unchanged individual settings;
- collection replacement and synchronization preserve the collection flag;
- management still lists and permits editing all member sources while their collection is disabled;
- existing shelf books continue catalog, content, image, and source-bound reading operations through their current BookSource;
- the collection control and its explanation/status/error feedback are localized in English, Simplified Chinese, and Traditional Chinese;
- the implementation introduces no bulk source rewrite, migration framework, generic policy engine, or frontend duplication of backend eligibility rules.

Example invariant:

```text
Collection 1 owns A, B, C
saved source state: A=false, B=false, C=true
collection disabled: A/B/C absent from Search and Explore
collection re-enabled: only C returns
```

## Scope

Included:

- persisted `Collection.Enabled` state;
- fresh reader schema epoch 7;
- effective Search and Explore eligibility at the BookSource persistence seam;
- direct Explore-open enforcement for stale clients;
- collection PATCH support for availability;
- collection-management UI toggle, disabled state, localized explanation, and feedback;
- focused persistence, API, eligibility, and UI tests;
- current architecture and project-state documentation updates when implementation completes.

Excluded:

- changing `BookSource.Enabled` or `BookSource.EnabledExplore` when a collection changes;
- blocking existing shelf books from Book Info, catalog, content, image, cache, or source-bound reading operations;
- blocking source interaction/login/settings maintenance;
- pausing collection synchronization or changing sync schedules;
- adding a general feature-flag, policy, capability, or collection hierarchy framework;
- schema migration machinery or preservation of pre-public development reader databases;
- standalone-source grouping or bulk actions unrelated to collection availability;
- redesigning the Source Management page beyond the focused control and state communication.

## Accepted Approach

### State model

Add `enabled INTEGER NOT NULL DEFAULT 1` to `book_source_collections` and expose it as `Collection.Enabled` / `SourceCollection.enabled`.

The collection flag is reader-owned management state. It is not part of imported BookSource JSON and is not rewritten by collection synchronization or replacement.

The authoritative discovery rule is:

```text
effectively enabled for Search =
  source.enabled
  AND (source is standalone OR source.collection.enabled)

effectively enabled for Explore =
  source.enabled
  AND source.enabledExplore
  AND source has an Explore definition
  AND (source is standalone OR source.collection.enabled)
```

### Backend seam

Keep collection policy inside `backend/internal/booksource/`:

- `Store.List()` remains an unfiltered management listing;
- `Store.ListEnabled()` applies collection availability for Search;
- `Store.ListExploreEnabled()` applies collection availability for Explore;
- add a small effective-availability lookup for direct source entry points that cannot rely on a prior list;
- `book.Searcher` consumes those store results and does not learn collection query details.

Direct `OpenExplore(sourceID)` must reject a member of a disabled collection even if a client retained an old source ID. This closes a stale-client bypass while keeping the policy localized.

Existing shelf workflows intentionally continue to use `GetByID` and exact stored bindings without the discovery gate.

### Update interface

Extend the existing authenticated collection PATCH interface with optional `enabled`:

```json
{ "enabled": false }
```

An update remains valid when at least one of `name`, `syncInterval`, or `enabled` is present. Store updates change only collection state and `updated_at`; they do not update member source rows or close source sessions because existing shelf reading remains valid.

### Frontend ownership

The Source Management feature owns collection presentation:

- place one switch in the selected collection detail/actions region;
- label it as collection availability rather than as a member-source setting;
- explain that it controls Search and Explore while preserving individual source settings;
- keep member source switches visible and bound to their saved values;
- visually identify disabled collections without implying their source definitions were disabled or deleted;
- prevent duplicate updates while the collection mutation is in flight;
- reload authoritative collection/source state after success and show localized feedback;
- refresh Explore state after a collection availability change so stale visible catalogs do not remain presented as available.

The frontend may use collection state for management display and effective counts, but backend store queries remain authoritative for actual Search/Explore eligibility.

### Schema policy

Increment `CurrentReaderSchemaVersion` from 6 to 7. This repository has no external users yet and pre-public reader data is disposable, so a schema mismatch may be resolved using `docs/runbooks/development-data-reset.md` and re-importing BookSources.

Do not restart numbering at 1: schema epochs remain monotonic so every persisted shape has an unambiguous identifier. Do not introduce migrations solely for this field.

## Decisions

### Separate collection policy from source preferences

**Decision:** Persist one collection-level availability flag and combine it with source preferences at read time.

**Why:** This exactly preserves the user's A/B/C state across temporary collection disablement and keeps collection policy cohesive.

**Alternatives:** Bulk-writing every member's `enabled` flag would destroy individual intent or require a second snapshot of prior state. A generic policy layer would be speculative for one boolean gate.

**Revisit when:** Collections gain several independent execution policies that can no longer be expressed clearly through focused fields and queries.

### Discovery-only behavior

**Decision:** Collection availability affects Search and Explore only.

**Why:** The requested purpose is temporary discovery management. Existing shelf books have an exact source binding and should remain readable; treating a collection toggle as revocation would expand checks across unrelated workflows and create surprising data-access behavior.

**Alternatives:** Blocking every source operation was rejected as broader, more coupled, and unsupported by the current product requirement.

**Revisit when:** Users need an explicit security/offline/revocation control. That should be a separately named policy with deliberately defined effects.

### Effective eligibility in store queries

**Decision:** Apply the gate in BookSource store queries and a direct effective lookup, not independently in Search, Explore, and Vue.

**Why:** Search and Explore already receive candidates from narrow store interfaces. One SQL-owned rule gives locality, consistent behavior, and focused tests.

**Alternatives:** Filtering in each workflow or only in Vue would duplicate policy and permit bypasses.

**Revisit when:** Candidate eligibility becomes dynamic or depends on runtime state that cannot be represented reliably in persistence queries.

### Fresh schema epoch

**Decision:** Use reader schema version 7 without migration machinery.

**Why:** There are no external users, development data can be reset safely, and this follows the repository's explicit pre-public schema policy.

**Alternatives:** Reusing version 1 creates ambiguous database identities. A v6-to-v7 migration adds maintenance and test surface without a real compatibility requirement.

**Revisit when:** Preserving reader databases across releases becomes a product commitment.

## Progress

- [x] Inspect collection persistence, replacement/sync behavior, Search/Explore candidate seams, direct Explore opening, schema policy, API, and management UI.
- [x] Confirm discovery-only behavior with the user.
- [x] Record accepted design and implementation boundaries.
- [x] Add collection state and schema epoch 7.
- [x] Apply effective availability to Search and Explore store access.
- [x] Extend collection PATCH behavior.
- [x] Add localized management UI and effective state display.
- [x] Run complete backend/frontend verification and visible interaction verification.
- [x] Update current architecture/project state and mark this plan complete.

## Current State

Completed on `feat/source-collection-availability`.

Delivered:

- `book_source_collections.enabled` defaults to true and reader schema epoch is 7;
- `Collection.Enabled` is returned by collection listing/get/mutation responses and preserved by sync/replacement;
- `Store.ListEnabled`, `Store.ListExploreEnabled`, and `Store.GetExploreEnabledByID` own effective discovery eligibility through one shared collection-aware persistence predicate;
- direct Explore opening uses the effective lookup, while shelf-reading callers retain unrestricted `GetByID` behavior;
- collection PATCH accepts optional `enabled` without changing member source rows;
- `SourceManagementView.vue` exposes a localized collection availability switch, paused state, effective aggregate counts, member-state preservation, busy/error feedback, and Explore-store refresh;
- English, Simplified Chinese, and Traditional Chinese strings are present;
- desktop and mobile interaction checks confirmed immediate removal/restoration and preserved source settings.

## Next Action

None for this completed workstream. Use a new plan if collection policy expands beyond discovery availability.

## Verification

Verified during planning:

- branch created from clean `main`: `feat/source-collection-availability`;
- Search candidate path: `Store.ListEnabled` → `Searcher.searchCandidates`;
- Explore candidate path: `Store.ListExploreEnabled` → `Searcher.ExploreSources`;
- direct Explore was identified as a bypass risk and was moved to the effective-availability lookup;
- collection replacement updates owned source rows but not collection policy fields;
- repository schema policy permits recreation of pre-public development reader data.

Final verification completed:

- complete backend suite passes: `go test ./...` (all packages);
- complete frontend suite passes: 50 test files;
- focused frontend API and management-view tests pass;
- frontend TypeScript check passes: `vue-tsc --noEmit`;
- production Vite build succeeds;
- UI detector reports no findings for `SourceManagementView.vue`;
- scoped static inspection reports no diagnostics, import cycles, or duplicated lines (Go diagnostics covered by package tests because `gopls` is unavailable);
- desktop and 390×844 mobile browser checks confirmed the switch placement and paused communication;
- pausing changed effective enabled/Search/Explore counts to zero and `/api/explore/sources` to `[]` while source checkboxes and persisted source rows retained their individual values;
- re-enabling restored only the individually enabled source to Explore;
- `git diff --check` passes.

## Open Questions

None.
