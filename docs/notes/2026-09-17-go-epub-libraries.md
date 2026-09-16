# Go EPUB reader libraries: bounded server import

Status: historical preimplementation research. The subsequent build proof and user decision to use focused Go parsing are recorded in the [active EPUB plan](../plans/2026-09-17-epub-support.md#current-state). The proposed Readium evaluation below was performed and is no longer an open dependency choice.

## Recommendation

**Make Readium's EPUB parser the sole third-party candidate for the first bounded contract proof, before committing to a home-grown package/navigation parser.** It provides materially reusable EPUB 2/3 behavior and focused tests, unlike the smaller candidates below. Do not adopt its default archive pipeline unchanged: enforce local-only, bounded, cancellable resource access behind its existing `Fetcher` interface, then translate its output into NovelReader-owned records. If that boundary or required diagnostics cannot be made reliable without substantial fork/duplication, prefer the standard-library baseline over repairing several incomplete libraries.

This is a proposed evaluation, not authorization to implement or add a dependency. None of these libraries provides NovelReader's semantic prose contract, anchor/footnote lifecycle, import receipts or safe resource-serving policy. Those remain application responsibilities; adopting a publication parser need not adopt its renderer, service model or persistence.

## Scope and evidence

Four actual Go reading/parsing candidates were inspected at the revisions below. Search identified repositories; conclusions come from first-party source, license files and checked-in tests. Repository commit timestamps are maintenance observations, **not** guarantees of support. No downloaded code was built or executed; no compatibility, performance, security or passing-test claim is made. Only the indicated source paths were reviewed, not entire dependency graphs.

| Candidate / pinned revision | Latest inspected commit date | License |
|---|---|---|
| [readium/go-toolkit](https://github.com/readium/go-toolkit/tree/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4) `e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4` | 2026-09-14 | [BSD-3-Clause](https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/LICENSE) |
| [taylorskalyo/goreader/epub](https://github.com/taylorskalyo/goreader/tree/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/epub) `f9256af1ef9f6d611c8c652e0f05d0b7364aabcc` | 2025-03-14 | [MIT](https://github.com/taylorskalyo/goreader/blob/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/LICENSE) |
| [ArcadiaLin/go-epub](https://github.com/ArcadiaLin/go-epub/tree/f0c64062a808381ff383c8f977d0f7814fa8795c) `f0c64062a808381ff383c8f977d0f7814fa8795c` | 2025-10-09 | [MIT](https://github.com/ArcadiaLin/go-epub/blob/f0c64062a808381ff383c8f977d0f7814fa8795c/LICENSE) |
| [kapmahc/epub](https://github.com/kapmahc/epub/tree/7759d080da6dd736e42fc896da65cc6f7e79c2b3) `7759d080da6dd736e42fc896da65cc6f7e79c2b3` | 2016-10-26 | [MIT](https://github.com/kapmahc/epub/blob/7759d080da6dd736e42fc896da65cc6f7e79c2b3/LICENSE) |

EPUB generation APIs are not reader substitutes. `goreader` is a terminal application, but its `epub` package is independently importable. Readium Go is a server-side publication toolkit, not an embedded browser renderer. No browser rendering stack is proposed here.

## 1. Readium: strongest parsing reuse, nontrivial boundary cost

- **API and ownership:** `epub.NewParser(...).Parse(ctx, asset, fetcher)` returns a publication builder. `NewGoZIPArchive(*zip.Reader, closer, minimizeReads)` accepts a caller-created ZIP reader; the archive factory also accepts `ReaderAtCloser` plus size. `ArchiveFetcher.Close` closes its archive. This permits retaining the original ZIP without an extraction tree and offers an existing resource boundary rather than requiring a new plugin framework. [Parser][r-parser], [ZIP adapter][r-zip], [fetcher][r-fetcher].
- **Format and URLs:** OPF parsing, spine-derived linear reading order, separate non-reading-order resources, EPUB 2 NCX and EPUB 3 `nav` property selection are implemented. Navigation preserves hierarchy and resolves EPUB URLs relative to the navigation document, not just the OPF directory; archive lookup uses the decoded URL path. This is substantially more useful than string/path joining. It is not proof of NovelReader's confinement/duplicate-name policy. [Parser][r-parser], [factory][r-factory], [nav][r-nav], [NCX][r-ncx], [fetcher][r-fetcher].
- **Observed bounds/cancellation gap:** parsing does not eagerly read every chapter, but `ReadResourceAsXML` requests an entire resource and constructs an XML tree. ZIP `Read(0,0)` allocates `UncompressedSize64` bytes; optimization paths also allocate declared compressed sizes. `entryResource.Read/Stream` accept `ctx` but forward to archive entry methods **without context**. A context-shaped API therefore does not establish decompression cancellation. Byte, node/depth and total-work limits are not supplied by these paths. [XML helper][r-resource], [ZIP adapter][r-zip], [fetcher][r-fetcher].
- **Diagnostics and footprint:** navigation read/parse failures return an empty result rather than an error from `parseNavigationData`; a limit/cancellation failure there must not become a successful import with merely missing TOC. `go.mod` requires Go 1.25.8 and includes cloud, media and PDF dependencies; exact compiled dependency footprint was not measured. Selective package use does not imply a tiny module graph. [Parser][r-parser], [module][r-mod].
- **Test evidence:** dedicated package, nav, NCX, encryption and ZIP stream tests exist. Inspected nav tests assert nested navigation discovery, whitespace handling and nested label markup. These are stronger format evidence than an example-book smoke test, not hostile-input assurance. [EPUB test directory][r-tests], [nav tests][r-nav-tests].

## 2. goreader/epub: attractive small API, insufficient EPUB contract

`NewReader(io.ReaderAt, size)` leaves underlying input ownership with the caller; `OpenReader(path)` owns a file and returns a closer. Manifest `Item.Open()` streams an entry, so chapters/assets are lazy. However initialization uses unbounded `io.ReadAll` for OPF/NCX/nav; no context or entry-read interception hook is exposed. Manifest links use `path.Join`, not URL resolution. The model omits spine `linear` and manifest properties, and navigation discovery assumes literal manifest IDs `ncx` and `toc`. This cannot reliably implement arbitrary EPUB 2/3 navigation or auxiliary reading semantics without modifying/replacing core parsing. `OpenReader` also leaves its opened file unclosed on ZIP/init error paths. Sources: [reader][g-reader], [package/resources][g-package], [NCX][g-ncx], [nav][g-nav].

Checked-in tests verify metadata and manifest/spine bindings against one Alice EPUB; they do not demonstrate varied nav discovery or untrusted-input behavior. [Tests][g-tests]. **Not recommended for this scope:** repairing the omitted format rules reduces the value of adopting its small API.

## 3. ArcadiaLin/go-epub: extraction-oriented, loses required semantics

`ReadBook(path)` opens and closes the ZIP internally, reads container/OPF/TOC completely and eagerly parses spine chapters. Missing spine mappings/files are silently skipped. There is no context argument or live resource opener on the returned chapter model. `Chapter` keeps paragraph strings and image paths, not mixed inline structure, IDs or link targets. Public lower-level parsers exist, but using them would bypass much of the convenient API. [Reader][a-reader], [chapter][a-chapter].

Both NCX and EPUB 3 nav parsing exist, but `ParseTOC` passes the OPF directory as the link base even when the navigation file lives in a subdirectory. Navigation lookup also uses `path.Clean`; this is not complete URI handling. XML parsing sets `Strict=false`. No `_test.go` files were present in the inspected revision; bundled EPUB examples are not assertions. [TOC][a-toc], [reader][a-reader], [repository][a-root]. **Not recommended:** eager/lossy extraction and failure semantics conflict with the accepted reading profile.

## 4. kapmahc/epub: minimal lazy reader, too weak a boundary

`Open(path)` owns a ZIP; `Book.Close`, `Book.Files` and `Book.Open(relativeName)` expose lazy resource reading without extraction. OPF and spine-selected NCX are decoded, but there is no EPUB 3 nav parser in the inspected package, no `ReaderAt` constructor and no context/budget API. XML decoding streams but is unbounded; `readBytes` uses `ioutil.ReadAll`. Crucially, `readXML` returns `nil` when opening a required entry fails, and `readBytes` similarly returns `(nil,nil)`. Resource names are OPF-relative `path.Join` results, not resolved URLs. [Open][k-open], [book][k-book].

Its sole test opens `test.epub`, which is absent from the inspected tree, and logs rather than asserting publication behavior. The helper also closes the book before returning it. [Test][k-test]. **Not recommended:** missing-file handling and missing EPUB 3 behavior require substantive repair.

## Standard-library baseline: reuse primitives, own EPUB policy

NovelReader already pins `golang.org/x/net v0.56.0` and `golang.org/x/text v0.38.0` in `backend/go.mod`; using existing parsing primitives does not require inventing ZIP, XML or HTML parsers. [Go ZIP](https://pkg.go.dev/archive/zip#NewReader) accepts `io.ReaderAt` and exposes streamed entry reads. [XML Decoder](https://pkg.go.dev/encoding/xml#Decoder) supports token decoding, entity mappings and charset readers. These primitives are not an EPUB package/navigation implementation: a custom solution still owns OPF/NCX/nav interpretation, EPUB URL resolution, compatibility tests and diagnostics.

The [HTML package documentation](https://pkg.go.dev/golang.org/x/net/html#hdr-Security_Considerations) distinguishes tokenization from tree construction. It is an HTML parser, not proof of XHTML namespace conformance; parser selection and EPUB 2 entities/UTF encodings must be exercised explicitly. `Tokenizer.SetMaxBuf` bounds token buffering, not an entire document or DOM. The fetched public documentation described Go 1.27.1 and x/net v0.59.0; this was API-level inspection, not execution against the repository's pinned versions.

Neither stdlib ZIP path handling nor an EPUB library replaces application confinement and resource budgets. Equally, avoid building a custom ZIP central-directory decoder merely to obtain another limit: start with compressed-input limits and bounded use of maintained ZIP primitives, then measure the actual exposure before adding more machinery.

**Decision criterion:** adopt Readium only if its reused format behavior materially exceeds the adapter, dependency and diagnostic-repair cost. Small custom EPUB glue over established primitives can be cleaner than a maintained fork, but has real format-maintenance cost. Do not implement two production parsers, add a runtime parser selector, or expose Readium models in database/API contracts. Revisit custom parsing if the proof requires duplicating the package/navigation logic it was supposed to save.

## Contract-proof gate and remaining uncertainty

Keep the existing proposed `internal/epub` ownership boundary: ZIP/resource policy, package/navigation interpretation and semantic normalization are cohesive concerns, but third-party types must not escape into storage, HTTP or the prose schema.

Before accepting Readium, demonstrate with small synthetic EPUB 2/3 fixtures that a bounded local fetcher prevents allocations/decompression beyond the budget; cancellation/limit failures remain fatal even when upstream treats optional metadata/navigation failures as absence; nested URL bases and fragments survive translation; auxiliary note targets stay separate from the main sequence. Check XML tree depth/node limits as well as byte limits. Do not assume a timed-out goroutine stops parsing. Validate the actual build graph and toolchain impact separately.

Readium's direct exported package/nav/NCX functions are a possible narrower reuse seam, but their integration cost was not demonstrated here. None of the candidates was verified against NovelReader's full DRM/fixed-layout rejection, duplicate archive names, footnotes, ruby, table semantics or hostile EPUB requirements. A parsing library reduces format work; it does not remove these acceptance checks.

[r-parser]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/parser/epub/parser.go
[r-zip]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/archive/archive_zip.go
[r-fetcher]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/fetcher/fetcher_archive.go
[r-resource]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/fetcher/resource.go
[r-factory]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/parser/epub/factory.go
[r-nav]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/parser/epub/parser_navdoc.go
[r-ncx]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/parser/epub/parser_ncx.go
[r-mod]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/go.mod
[r-tests]: https://github.com/readium/go-toolkit/tree/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/parser/epub
[r-nav-tests]: https://github.com/readium/go-toolkit/blob/e36e46c88ad82c2ff9bfabc5657d988bd8c99cd4/pkg/parser/epub/parser_navdoc_test.go
[g-reader]: https://github.com/taylorskalyo/goreader/blob/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/epub/epub.go
[g-package]: https://github.com/taylorskalyo/goreader/blob/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/epub/package.go
[g-nav]: https://github.com/taylorskalyo/goreader/blob/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/epub/nav.go
[g-ncx]: https://github.com/taylorskalyo/goreader/blob/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/epub/ncx.go
[g-tests]: https://github.com/taylorskalyo/goreader/blob/f9256af1ef9f6d611c8c652e0f05d0b7364aabcc/epub/epub_test.go
[a-root]: https://github.com/ArcadiaLin/go-epub/tree/f0c64062a808381ff383c8f977d0f7814fa8795c
[a-reader]: https://github.com/ArcadiaLin/go-epub/blob/f0c64062a808381ff383c8f977d0f7814fa8795c/reader.go
[a-chapter]: https://github.com/ArcadiaLin/go-epub/blob/f0c64062a808381ff383c8f977d0f7814fa8795c/chapter.go
[a-toc]: https://github.com/ArcadiaLin/go-epub/blob/f0c64062a808381ff383c8f977d0f7814fa8795c/toc.go
[k-open]: https://github.com/kapmahc/epub/blob/7759d080da6dd736e42fc896da65cc6f7e79c2b3/open.go
[k-book]: https://github.com/kapmahc/epub/blob/7759d080da6dd736e42fc896da65cc6f7e79c2b3/book.go
[k-test]: https://github.com/kapmahc/epub/blob/7759d080da6dd736e42fc896da65cc6f7e79c2b3/epub_test.go
