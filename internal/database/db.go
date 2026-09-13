// Package database handles the SQLite connection and schema bootstrap.
package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open opens (creating if necessary) the SQLite database at path and applies
// the schema. It returns a ready-to-use *sql.DB.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite is a single writer; keep the pool small and stable.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Hour)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// migrate applies incremental changes to databases created by older versions.
func migrate(db *sql.DB) error {
	if !hasColumn(db, "incomes", "external_id") {
		if _, err := db.Exec(`ALTER TABLE incomes ADD COLUMN external_id TEXT`); err != nil {
			return err
		}
	}
	if !hasColumn(db, "incomes", "voided") {
		if _, err := db.Exec(`ALTER TABLE incomes ADD COLUMN voided INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !hasColumn(db, "incomes", "sale_date") {
		if _, err := db.Exec(`ALTER TABLE incomes ADD COLUMN sale_date DATE`); err != nil {
			return err
		}
	}
	_, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_incomes_external
		ON incomes(external_id) WHERE external_id IS NOT NULL`)
	return err
}

func hasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil && name == column {
			return true
		}
	}
	return false
}
