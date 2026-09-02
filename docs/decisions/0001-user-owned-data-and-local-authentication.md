---
status: accepted
---

# Use local accounts with self-contained portable reader directories

NovelReader will use local username/password Reader Accounts with opaque server-side sessions. Deployment authentication and administration live in `system.db`; each account’s meaningful Reader Data lives in a self-contained `data/users/<immutable-user-id>/` directory with an unencrypted `reader.db` and ordinary files. Reversible source credentials are isolated in a separate encrypted credential store and are never included in portable per-reader exports.

This was chosen over one global Reader Data database because sources and reading activity must be isolated between readers while remaining inspectable, movable, and recoverable if authentication, administrator access, the application secret, or the application itself fails. With NovelReader stopped, copying the complete `data/` directory is a supported deployment backup. Losing the source-credential key loses only external source logins, never books, sources, progress, history, bookmarks, caches, preferences, or files.

The first Administrator is created through a one-time browser setup token; roles are stored in `system.db`, public registration creates only readers, and a temporary environment recovery token can restore Administrator access without rewriting Reader Data. Authentication and conversion away from globally keyed stores must ship as one fail-closed cutover. Feature code accesses reader storage through one reader-storage module rather than hardcoded paths, allowing future reader-owned schemas and files to be added without coupling them to authentication or HTTP handlers.
