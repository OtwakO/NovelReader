package auth

const systemSchema = `
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    username_normalized TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('reader', 'admin')),
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'deleting')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    auth_version INTEGER NOT NULL DEFAULT 1 CHECK (auth_version > 0)
);

CREATE TABLE auth_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash BLOB NOT NULL UNIQUE CHECK (typeof(token_hash) = 'blob' AND length(token_hash) = 32),
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX auth_sessions_user_id_idx ON auth_sessions(user_id);

CREATE TABLE password_reset_tokens (
    token_hash BLOB PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_by_user_id TEXT,
    created_by_username TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    used_at INTEGER,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens(user_id);
CREATE INDEX password_reset_tokens_expires_at_idx ON password_reset_tokens(expires_at);

CREATE TABLE setup_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    status TEXT NOT NULL CHECK (status IN ('open', 'claimed', 'closed')),
    proposed_user_id TEXT,
    username TEXT,
    username_normalized TEXT,
    password_hash TEXT,
    claimed_at INTEGER,
    claim_expires_at INTEGER,
    closed_at INTEGER,
    CHECK (
        (status = 'open' AND proposed_user_id IS NULL AND username IS NULL AND username_normalized IS NULL AND password_hash IS NULL AND claimed_at IS NULL AND claim_expires_at IS NULL AND closed_at IS NULL)
        OR (status = 'claimed' AND proposed_user_id IS NOT NULL AND username IS NOT NULL AND username_normalized IS NOT NULL AND password_hash IS NOT NULL AND claimed_at IS NOT NULL AND claim_expires_at IS NOT NULL AND closed_at IS NULL)
        OR (status = 'closed' AND proposed_user_id IS NOT NULL AND username IS NULL AND username_normalized IS NULL AND password_hash IS NULL AND claimed_at IS NOT NULL AND claim_expires_at IS NOT NULL AND closed_at IS NOT NULL)
    )
);
INSERT INTO setup_state (id, status) VALUES (1, 'open');

CREATE TABLE admin_recovery_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    status TEXT NOT NULL CHECK (status IN ('idle', 'claimed', 'completed')),
    action TEXT CHECK (action IN ('reset_existing', 'create_replacement')),
    generation TEXT,
    user_id TEXT,
    username TEXT,
    username_normalized TEXT,
    password_hash TEXT,
    home_provisioning INTEGER NOT NULL DEFAULT 0 CHECK (home_provisioning IN (0, 1)),
    claimed_at INTEGER,
    completed_at INTEGER,
    CHECK (
        (status = 'idle' AND action IS NULL AND generation IS NULL AND user_id IS NULL AND username IS NULL AND username_normalized IS NULL AND password_hash IS NULL AND home_provisioning = 0 AND claimed_at IS NULL AND completed_at IS NULL)
        OR (status = 'claimed' AND action IS NOT NULL AND generation IS NOT NULL AND user_id IS NOT NULL AND username IS NOT NULL AND username_normalized IS NOT NULL AND password_hash IS NOT NULL AND claimed_at IS NOT NULL AND completed_at IS NULL AND (action = 'create_replacement' OR home_provisioning = 0))
        OR (status = 'completed' AND action IS NOT NULL AND generation IS NOT NULL AND user_id IS NOT NULL AND username IS NULL AND username_normalized IS NULL AND password_hash IS NULL AND home_provisioning = 0 AND claimed_at IS NOT NULL AND completed_at IS NOT NULL)
    )
);
INSERT INTO admin_recovery_state (id, status) VALUES (1, 'idle');

-- No foreign key to users: this durable job must survive removal of the account row.
CREATE TABLE account_deletions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'removing_data', 'removing_account', 'complete', 'failed')),
    last_error TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
);
CREATE INDEX account_deletions_status_idx ON account_deletions(status);
`
