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

	// 8: multi-user ownership and per-user resource names.
	`-- sb-fox:disable-foreign-keys
	CREATE TABLE users (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		username       TEXT UNIQUE NOT NULL,
		password_hash  TEXT NOT NULL,
		role           TEXT NOT NULL CHECK (role IN ('admin', 'user')),
		node_limit     INTEGER NOT NULL DEFAULT 0,
		profile_limit  INTEGER NOT NULL DEFAULT 0,
		template_limit INTEGER NOT NULL DEFAULT 0,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	);
	INSERT INTO users (id, username, password_hash, role, node_limit, profile_limit, template_limit, created_at, updated_at)
		SELECT id, username, password_hash, 'admin', 0, 0, 0, created_at, updated_at FROM admin;

	CREATE TABLE templates_new (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name          TEXT NOT NULL,
		kind          TEXT NOT NULL,
		content       TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		UNIQUE(owner_user_id, name)
	);
	INSERT INTO templates_new (id, owner_user_id, name, kind, content, description, created_at, updated_at)
		SELECT id, 1, name, kind, content, description, created_at, updated_at FROM templates;

	CREATE TABLE subscription_sources_new (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name          TEXT NOT NULL,
		url           TEXT NOT NULL,
		last_fetch_at TEXT,
		last_status   TEXT NOT NULL DEFAULT '',
		node_count    INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL
	);
	INSERT INTO subscription_sources_new (id, owner_user_id, name, url, last_fetch_at, last_status, node_count, created_at)
		SELECT id, 1, name, url, last_fetch_at, last_status, node_count, created_at FROM subscription_sources;

	CREATE TABLE nodes_new (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
	);
	INSERT INTO nodes_new (id, owner_user_id, tag, type, server, server_port, country_code, country_source, source, source_ref, has_detour, detour, raw, created_at, updated_at)
		SELECT id, 1, tag, type, server, server_port, country_code, country_source, source, source_ref, has_detour, detour, raw, created_at, updated_at FROM nodes;

	CREATE TABLE profiles_new (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name          TEXT NOT NULL,
		template_id   INTEGER NOT NULL REFERENCES templates(id),
		options       TEXT NOT NULL DEFAULT '{}',
		token         TEXT UNIQUE NOT NULL,
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		UNIQUE(owner_user_id, name)
	);
	INSERT INTO profiles_new (id, owner_user_id, name, template_id, options, token, created_at, updated_at)
		SELECT id, 1, name, template_id, options, token, created_at, updated_at FROM profiles;

	CREATE TABLE profile_nodes_new (
		profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
		node_id    INTEGER NOT NULL REFERENCES nodes(id)    ON DELETE CASCADE,
		position   INTEGER NOT NULL,
		PRIMARY KEY (profile_id, node_id)
	);
	INSERT INTO profile_nodes_new (profile_id, node_id, position)
		SELECT profile_id, node_id, position FROM profile_nodes;

	DROP TABLE profile_nodes;
	DROP TABLE profiles;
	DROP TABLE nodes;
	DROP TABLE subscription_sources;
	DROP TABLE templates;

	ALTER TABLE templates_new RENAME TO templates;
	ALTER TABLE subscription_sources_new RENAME TO subscription_sources;
	ALTER TABLE nodes_new RENAME TO nodes;
	ALTER TABLE profiles_new RENAME TO profiles;
	ALTER TABLE profile_nodes_new RENAME TO profile_nodes;

	CREATE INDEX idx_nodes_country ON nodes(country_code);
	CREATE INDEX idx_nodes_source ON nodes(source);
	CREATE INDEX idx_nodes_owner ON nodes(owner_user_id);
	CREATE INDEX idx_templates_owner ON templates(owner_user_id);
	CREATE INDEX idx_sources_owner ON subscription_sources(owner_user_id);
	CREATE INDEX idx_profiles_owner ON profiles(owner_user_id);`,

	// 9: reusable node groups and ordered profile node-group membership.
	`CREATE TABLE node_groups (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name          TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		UNIQUE(owner_user_id, name)
	);
	CREATE TABLE node_group_nodes (
		group_id INTEGER NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
		node_id  INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		PRIMARY KEY (group_id, node_id)
	);
	CREATE TABLE profile_node_groups (
		profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
		group_id   INTEGER NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
		position   INTEGER NOT NULL,
		PRIMARY KEY (profile_id, group_id)
	);
	CREATE INDEX idx_node_groups_owner ON node_groups(owner_user_id);`,

	// 10: shared per-user public subscription token.
	`ALTER TABLE users ADD COLUMN subscription_token TEXT NOT NULL DEFAULT '';
	UPDATE users SET subscription_token = lower(hex(randomblob(16))) WHERE subscription_token = '';
	CREATE UNIQUE INDEX idx_users_subscription_token ON users(subscription_token);`,

	// 11: per-user preferred sing-box kernel profile.
	`ALTER TABLE users ADD COLUMN active_kernel_id TEXT NOT NULL DEFAULT '';`,

	// 12: per-user country heat order override.
	`ALTER TABLE users ADD COLUMN country_heat_order TEXT NOT NULL DEFAULT '';`,

	// 13: per-profile public subscription switch.
	`ALTER TABLE profiles ADD COLUMN subscription_enabled INTEGER NOT NULL DEFAULT 1;`,
}
