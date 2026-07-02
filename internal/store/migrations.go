package store

// migrations holds ordered DDL statements. Each entry is applied once, in
// order, and recorded in schema_migrations. Append new migrations; never edit
// an already-shipped one.
var migrations = []string{
	// 1: bookkeeping
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	);`,

	// 2: single admin (row id enforced = 1 by app); users reserved, unused
	`CREATE TABLE admin (
		id            INTEGER PRIMARY KEY CHECK (id = 1),
		username      TEXT NOT NULL DEFAULT 'admin',
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	);`,

	// 3: key/value settings
	`CREATE TABLE settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`,

	// 4: templates (builtin seeded + user imported)
	`CREATE TABLE templates (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT UNIQUE NOT NULL,
		kind        TEXT NOT NULL,
		content     TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);`,

	// 5: subscription sources
	`CREATE TABLE subscription_sources (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT NOT NULL,
		url           TEXT NOT NULL,
		last_fetch_at TEXT,
		last_status   TEXT NOT NULL DEFAULT '',
		node_count    INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL
	);`,

	// 6: nodes (raw outbound blob + extracted metadata)
	`CREATE TABLE nodes (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		tag            TEXT NOT NULL,
		type           TEXT NOT NULL,
		server         TEXT NOT NULL DEFAULT '',
		server_port    INTEGER NOT NULL DEFAULT 0,
		country_code   TEXT NOT NULL DEFAULT '',
		country_source TEXT NOT NULL DEFAULT 'auto',
		source         TEXT NOT NULL,
		source_ref     INTEGER REFERENCES subscription_sources(id) ON DELETE SET NULL,
		has_detour     INTEGER NOT NULL DEFAULT 0,
		detour         TEXT NOT NULL DEFAULT '',
		raw            TEXT NOT NULL,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	);`,
	`CREATE INDEX idx_nodes_country ON nodes(country_code);`,
	`CREATE INDEX idx_nodes_source  ON nodes(source);`,

	// 7: profiles + ordered node membership
	`CREATE TABLE profiles (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT UNIQUE NOT NULL,
		template_id INTEGER NOT NULL REFERENCES templates(id),
		options     TEXT NOT NULL DEFAULT '{}',
		token       TEXT UNIQUE NOT NULL,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);`,
	`CREATE TABLE profile_nodes (
		profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
		node_id    INTEGER NOT NULL REFERENCES nodes(id)    ON DELETE CASCADE,
		position   INTEGER NOT NULL,
		PRIMARY KEY (profile_id, node_id)
	);`,
}
