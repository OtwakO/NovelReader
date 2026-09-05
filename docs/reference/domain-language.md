# NovelReader Domain

NovelReader is a multi-user reading system in which each reader owns an isolated collection of sources, books, reading state, and source credentials.

## Language

**Reader Account**:
A person’s authenticated NovelReader identity, identified by a unique case-insensitive username.
_Avoid_: User profile, tenant, login

**Reader Data**:
Portable, unencrypted data owned by one Reader Account: imported sources, files, shelf entries, chapters and caches, progress, bookmarks, history, and preferences.
_Avoid_: Global library, shared catalog, credentials

**Reader Directory**:
The self-contained directory identified by an immutable Reader Account ID that holds one plaintext `reader.db`, ordinary files, and a separately protected credential store.
_Avoid_: Username directory, home folder

**System Data**:
Deployment-level account, role, password-hash, Application Session, reset-token, setup, recovery, and deletion-job records stored separately from Reader Data.
_Avoid_: Reader Data, global library

**Application Session**:
A revocable browser sign-in for one Reader Account, represented to the browser by an opaque cookie and stored server-side only as a token hash.
_Avoid_: Source session, login cookie

**Source Session**:
Mutable cookies, headers, variables, and request context used while one Reader Account interacts with one imported source or book workflow.
_Avoid_: Application session, browser account

**Source Login State**:
Encrypted, durable cookies or headers that let one Reader Account authenticate to one imported book source.
_Avoid_: Application credential, shared source account

**Administrator**:
A Reader Account with the stored `admin` role that may manage ordinary Reader Accounts. Initial web administration cannot disable, reset, or delete another Administrator.
_Avoid_: Shared admin password, environment username list

**Registration Invite Code**:
An optional deployment-wide admission secret checked only when creating a Reader Account. It is not an account credential and is never persisted with the account.
_Avoid_: Recovery code, password

**Password Reset Token**:
A short-lived, single-use secret issued by an Administrator so a reader can choose a replacement password without revealing that password to the Administrator.
_Avoid_: Temporary password, invite code

**Bootstrap Token**:
A temporary deployment secret that authorizes only the one-time browser creation of the first Administrator.
_Avoid_: Administrator password, Application Session

**Recovery Token**:
A temporary deployment secret that authorizes disaster recovery of Administrator access without decrypting, claiming, or changing Reader Data.
_Avoid_: Password Reset Token, master password

**Portable Export**:
A bundle of one Reader Account’s plaintext `reader.db` and ordinary files that becomes owned by the destination account on import. It excludes the separate source credential store and account authority, but does not sanitize secrets embedded in losslessly preserved source definitions; see [backup boundaries](../architecture/authentication-and-reader-storage.md#backup-boundaries).
_Avoid_: Deployment backup, credential export

**Cold Backup**:
A complete copy of the stopped deployment’s `data/` directory, including System Data, every Reader Directory, and encrypted source credentials.
_Avoid_: Portable Export, live database copy

## Reading language

**Content Provider**:
An implementation that obtains and interprets a readable publication from one origin, such as a BookSource or a future imported file. Provider identity does not determine reading layout.
_Avoid_: Renderer, content type, BookSource when referring to all origins

**Reading Section**:
One ordered, addressable part of a readable shelf item, such as a BookSource chapter, EPUB spine item, generated TXT section, manga chapter, or audiobook chapter.
_Avoid_: Chapter when referring to the cross-format architectural concept

**Reading Document**:
The normalized, provider-agnostic content opened for one Reading Section, with an explicit reading modality such as prose, image sequence, or audio.
_Avoid_: Raw source response, provider payload, universal media block list

**Prose Document**:
A Reading Document whose primary behavior is flowing text, optionally containing semantic inline resources such as illustrations.
_Avoid_: EPUB document, TXT document, generic chapter content

**Image-Sequence Document**:
A Reading Document whose ordered images are primary reading units and therefore own page progression, fitting, preloading, and page-oriented location behavior.
_Avoid_: Prose with many images, gallery block

**Content Resource**:
Reader-authorized binary material referenced opaquely by a Reading Document and resolved by NovelReader, such as an inline image, manga page, EPUB asset, or future audio stream.
_Avoid_: Upstream URL, proxy URL, local path

**Reading Session**:
The frontend workflow that opens Reading Sections, coordinates common Reader navigation and state, selects the renderer for each Reading Document, and persists reading location.
_Avoid_: Application Session, Source Session

**Reading Location**:
A modality-specific durable position within a Reading Section, such as prose progression, image page index, or audio time offset.
_Avoid_: Universal scroll percentage
