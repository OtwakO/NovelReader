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
A credential-free bundle of one Reader Account’s plaintext `reader.db` and ordinary files that becomes owned by the destination account on import.
_Avoid_: Deployment backup, credential export

**Cold Backup**:
A complete copy of the stopped deployment’s `data/` directory, including System Data, every Reader Directory, and encrypted source credentials.
_Avoid_: Portable Export, live database copy
