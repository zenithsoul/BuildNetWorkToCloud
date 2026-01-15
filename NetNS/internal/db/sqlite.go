package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQL database connection
type DB struct {
	*sql.DB
}

// DefaultDBPath returns the default database path
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "netns.db"
	}
	dir := filepath.Join(home, ".netns-mgr")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "netns.db")
}

// Open opens or creates the SQLite database
func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	wrapper := &DB{db}
	if err := wrapper.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return wrapper, nil
}

// migrate creates the database schema
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS namespaces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT
	);

	CREATE TABLE IF NOT EXISTS veth_pairs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		peer_name TEXT NOT NULL,
		ns_id INTEGER REFERENCES namespaces(id) ON DELETE CASCADE,
		peer_ns_id INTEGER REFERENCES namespaces(id) ON DELETE SET NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ip_addresses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		interface_name TEXT NOT NULL,
		ns_id INTEGER REFERENCES namespaces(id) ON DELETE CASCADE,
		address TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS routes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ns_id INTEGER REFERENCES namespaces(id) ON DELETE CASCADE,
		destination TEXT NOT NULL,
		gateway TEXT,
		interface_name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS bridges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		ns_id INTEGER REFERENCES namespaces(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS bridge_ports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bridge_id INTEGER REFERENCES bridges(id) ON DELETE CASCADE,
		interface_name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS gre_tunnels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		local_ip TEXT NOT NULL,
		remote_ip TEXT NOT NULL,
		gre_key INTEGER DEFAULT 0,
		ttl INTEGER DEFAULT 0,
		ns_id INTEGER REFERENCES namespaces(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_veth_ns ON veth_pairs(ns_id);
	CREATE INDEX IF NOT EXISTS idx_veth_peer_ns ON veth_pairs(peer_ns_id);
	CREATE INDEX IF NOT EXISTS idx_ip_ns ON ip_addresses(ns_id);
	CREATE INDEX IF NOT EXISTS idx_routes_ns ON routes(ns_id);
	CREATE INDEX IF NOT EXISTS idx_bridges_ns ON bridges(ns_id);
	CREATE INDEX IF NOT EXISTS idx_bridge_ports_bridge ON bridge_ports(bridge_id);
	CREATE INDEX IF NOT EXISTS idx_gre_tunnels_ns ON gre_tunnels(ns_id);
	`

	_, err := db.Exec(schema)
	return err
}
