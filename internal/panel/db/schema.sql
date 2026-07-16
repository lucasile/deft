CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT,
    last_seen INTEGER,
    cert_fingerprint TEXT,
    cert_subject TEXT,
    created_at INTEGER
);

CREATE TABLE IF NOT EXISTS containers (
    id TEXT PRIMARY KEY,
    node_id TEXT,
    name TEXT,
    image TEXT,
    status TEXT,
    FOREIGN KEY(node_id) REFERENCES nodes(id)
);

CREATE TABLE IF NOT EXISTS servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    node_id TEXT NOT NULL,
    container_id TEXT,
    image TEXT NOT NULL,
    status TEXT NOT NULL,
    desired_config_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(node_id) REFERENCES nodes(id),
    FOREIGN KEY(container_id) REFERENCES containers(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    username TEXT,
    action TEXT NOT NULL,
    node_id TEXT,
    target_id TEXT,
    command_id TEXT,
    success INTEGER NOT NULL,
    message TEXT,
    remote_addr TEXT,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS commands (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    action TEXT NOT NULL,
    target_id TEXT,
    status TEXT NOT NULL,
    success INTEGER,
    message TEXT,
    created_at INTEGER NOT NULL,
    completed_at INTEGER,
    FOREIGN KEY(node_id) REFERENCES nodes(id)
);

CREATE TABLE IF NOT EXISTS join_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    node_name TEXT,
    created_by TEXT,
    used_by_node_id TEXT,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    used_at INTEGER,
    revoked_at INTEGER,
    FOREIGN KEY(created_by) REFERENCES users(id),
    FOREIGN KEY(used_by_node_id) REFERENCES nodes(id)
);

CREATE TABLE IF NOT EXISTS join_requests (
    id TEXT PRIMARY KEY,
    secret_hash TEXT NOT NULL UNIQUE,
    verification_code TEXT NOT NULL,
    node_name TEXT,
    csr_pem TEXT NOT NULL,
    ca_cert_pem TEXT,
    cert_pem TEXT,
    approved_by TEXT,
    approved_node_id TEXT,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    approved_at INTEGER,
    denied_at INTEGER,
    FOREIGN KEY(approved_by) REFERENCES users(id),
    FOREIGN KEY(approved_node_id) REFERENCES nodes(id)
);
