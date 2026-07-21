// Package cache is the agent's local durable outbox. Collected envelopes are
// persisted to a SQLite database before shipping, so nothing is lost across
// network outages or restarts. It uses the pure-Go modernc.org/sqlite driver
// (no cgo) which keeps cross-compilation to Windows trivial.
package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Cache is a FIFO outbox backed by SQLite.
type Cache struct {
	db       *sql.DB
	maxQueue int
}

// Item is one queued message pending delivery.
type Item struct {
	ID      int64
	Kind    string // "envelope" | "heartbeat" | "command_result"
	Payload []byte
}

// Open initialises (and migrates) the outbox database at path.
func Open(path string, maxQueue int) (*Cache, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create cache dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}
	// A single writer keeps SQLite happy without WAL contention; the agent is
	// low-throughput so this is plenty.
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS outbox (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		kind       TEXT    NOT NULL,
		payload    BLOB    NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_outbox_id ON outbox(id);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Cache{db: db, maxQueue: maxQueue}, nil
}

// Enqueue appends a message to the outbox, trimming the oldest entries if the
// queue exceeds maxQueue (bounded memory/disk under prolonged outage).
func (c *Cache) Enqueue(kind string, payload []byte) error {
	if _, err := c.db.Exec(
		"INSERT INTO outbox(kind, payload, created_at) VALUES(?,?,?)",
		kind, payload, time.Now().Unix(),
	); err != nil {
		return err
	}
	return c.trim()
}

func (c *Cache) trim() error {
	if c.maxQueue <= 0 {
		return nil
	}
	_, err := c.db.Exec(`
		DELETE FROM outbox WHERE id IN (
			SELECT id FROM outbox ORDER BY id DESC LIMIT -1 OFFSET ?
		)`, c.maxQueue)
	return err
}

// Peek returns up to limit oldest items without removing them. The caller
// deletes them via Ack once the server confirms receipt.
func (c *Cache) Peek(limit int) ([]Item, error) {
	rows, err := c.db.Query(
		"SELECT id, kind, payload FROM outbox ORDER BY id ASC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Kind, &it.Payload); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// Ack removes delivered items by id.
func (c *Cache) Ack(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("DELETE FROM outbox WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// Count returns the number of queued items.
func (c *Cache) Count() (int, error) {
	var n int
	err := c.db.QueryRow("SELECT COUNT(*) FROM outbox").Scan(&n)
	return n, err
}

// Close closes the underlying database.
func (c *Cache) Close() error { return c.db.Close() }
