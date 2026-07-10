package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type DB struct {
	*sql.DB
}

func Init(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := configureSQLite(db); err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to configure sqlite %q: %w", pragma, err)
		}
	}

	db.SetConnMaxIdleTime(5 * time.Minute)

	return nil
}

func applyMigrations(db *sql.DB) error {
	hasCSRFToken, err := hasColumn(db, "sessions", "csrf_token")
	if err != nil {
		return err
	}
	if !hasCSRFToken {
		if _, err := db.Exec("ALTER TABLE sessions ADD COLUMN csrf_token TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("failed to add sessions.csrf_token: %w", err)
		}
	}

	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, fmt.Errorf("failed to inspect table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("failed to scan table info: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("failed to read table info: %w", err)
	}

	return false, nil
}
