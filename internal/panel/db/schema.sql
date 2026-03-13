CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT,
    last_seen INTEGER
);

CREATE TABLE IF NOT EXISTS containers (
    id TEXT PRIMARY KEY,
    node_id TEXT,
    name TEXT,
    image TEXT,
    status TEXT,
    FOREIGN KEY(node_id) REFERENCES nodes(id)
);
