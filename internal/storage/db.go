package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with application-specific helpers.
type DB struct {
	*sql.DB
}

// Open creates or connects to a SQLite database with production-appropriate pragmas.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory %s: %w", dir, err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", dbPath)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Set connection pool settings suitable for SQLite
	sqldb.SetMaxOpenConns(1) // SQLite performs best with serialized writes
	sqldb.SetMaxIdleConns(1)

	if err := sqldb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	db := &DB{DB: sqldb}
	if err := db.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("[STORAGE] Connected to SQLite database at %s", dbPath)
	return db, nil
}
