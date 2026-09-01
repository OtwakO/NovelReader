# Source Binding State

**Status:** Completed

## Goal

Represent every book source binding with one complete domain shape so metadata remains attached to the exact `(SourceID, BookURL)` binding whether it is active or alternate.

## Scope

- Replace the provisional active-entry-inside-an-alternate-array inference.
- Persist an explicit versioned binding-state JSON document in the existing `books.alternate_sources` column.
- Decode the legacy JSON array format without mutating it until the next normal write.
- Route binding enrichment, clearing, and source promotion through one store-owned mutation seam.
- Allow targeted search results matching the current binding to enrich active metadata.
- Preserve each binding's source-returned `lastChapter` snapshot as an opaque display hint.
- Present active, stored alternate, and newly discovered bindings through one unified frontend list.
- Preserve ordinary Search and BookSource execution behavior.

Out of scope:

- Parsing aggregate URLs, query syntax, provider names, or source variables.
- Inventing an internal-provider identity.
- Adding SQL columns or introducing general migration machinery.

## Accepted Approach

Use the existing complete `AltSource` data shape as the source-binding value and introduce one private `bindingState` aggregate:

- `Active AltSource`
- `Alternates []AltSource`

The retained `AltSource` name is API compatibility terminology; inside this design it represents either binding role.

Binding identity is exactly `(SourceID, BookURL)`. Optional metadata enriches an existing binding but never changes its identity. Discovery queries remain opaque display provenance.

Persist `BindingState` as a versioned JSON object in the existing `alternate_sources` column. Read legacy arrays as alternates plus an active binding reconstructed from the existing active book columns. Malformed JSON is a storage error.

All binding-state writes use a private store transaction/mutation helper. Source switching keeps binding promotion in the same transaction as active crawl state, chapters, progress, and bookmark migration.

## Decisions

- Keep the SQL schema unchanged because reader homes use exact epoch validation and the project intentionally has no migration machinery.
- Replace rather than layer over `persistedSourceBindings` and active-entry inference.
- Preserve query text only as provenance; never parse it or label it as a provider identity.
- The initial active binding is always materialized from current book fields. It gains richer metadata when available at shelf admission or when targeted search returns the exact current binding.
- `lastChapter` is retained verbatim per binding as source-returned display text. It is never parsed or treated as an internal-provider identity.
- Persistence keeps explicit active/alternate roles for transactional correctness; the frontend presents both roles as one known-bindings collection with current/stored/new presentation state.

## Current State

Completed. The store persists a versioned binding-state object, decodes legacy arrays, rejects malformed state, and routes merge/clear operations through one transaction-owned mutation seam. Source switching promotes the selected binding inside the same transaction as source fields, chapters, progress, and bookmark migration. Shelf admission materializes the active binding, and targeted searches can enrich the exact current binding.

Each binding now retains its source-returned `lastChapter` snapshot without interpreting it as provider identity. Search-result merging, candidate admission, exact-binding enrichment, and source promotion preserve that text with the exact `(sourceId, bookUrl)` binding.

Both Book Detail and Reader pass one complete shelf `Book` into the recovery module. The module derives one deduplicated known-bindings collection from the active binding, persisted alternates, and session discoveries. The active binding is rendered once with a current-source status and no switch action. Clear/rescan removes only persisted alternates and temporary discoveries; the active binding and its metadata remain visible.

No SQL schema change was required. Existing legacy arrays remain readable and are rewritten to the object format on the next normal binding-state write.

## Next Action

None. This plan is complete. Internal-provider identity remains opaque; `lastChapter` is only source-returned display text and is never parsed.

## Verification

Required focused scenario:

1. Create book with active A.
2. Enrich A and add alternates B and C.
3. Reload from SQLite.
4. Promote C, reload, and verify A/B/C metadata.
5. Promote B, reload, and verify metadata.
6. Promote A, reload, and verify metadata.
7. Clear alternates and verify active A metadata remains.
8. Decode legacy arrays and reject malformed binding-state JSON.
9. Run focused backend book/API tests, frontend recovery tests, typecheck, and production build.

Completed verification:

- Full backend suite: `go test ./...`
- Unified recovery UI: 13 focused tests across panel, reader sheet, and i18n
- Full frontend suite: 47 tests
- Frontend typecheck: `vue-tsc --noEmit`
- Frontend production build: `vite build`
- Transactional SQLite regression: A → C → B → A with reload after every promotion
- Legacy-array decoding, malformed-state rejection, active-metadata clearing, multi-result opaque-query, per-binding `lastChapter`, Search merge, and shelf-admission regressions
