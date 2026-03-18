package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const defaultTTL = 24 * time.Hour

// Cache provides SQLite-backed caching for API responses.
type Cache struct {
	db  *sql.DB
	ttl time.Duration
}

// Open opens or creates the cache database.
func Open() (*Cache, error) {
	dir, err := cacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	dbPath := filepath.Join(dir, "clickban.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			fetched_at INTEGER NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create cache table: %w", err)
	}

	return &Cache{db: db, ttl: defaultTTL}, nil
}

// Close closes the database.
func (c *Cache) Close() error {
	return c.db.Close()
}

// Get retrieves a cached value by key. Returns false if not found or expired.
func (c *Cache) Get(key string, dest interface{}) (bool, error) {
	var value string
	var fetchedAt int64
	err := c.db.QueryRow("SELECT value, fetched_at FROM cache WHERE key = ?", key).Scan(&value, &fetchedAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	age := time.Since(time.UnixMilli(fetchedAt))
	if age > c.ttl {
		return false, nil
	}

	if err := json.Unmarshal([]byte(value), dest); err != nil {
		return false, err
	}
	return true, nil
}

// Set stores a value in the cache.
func (c *Cache) Set(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		"INSERT OR REPLACE INTO cache (key, value, fetched_at) VALUES (?, ?, ?)",
		key, string(data), time.Now().UnixMilli(),
	)
	return err
}

func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "clickban"), nil
}
