package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateAddsExternalID reproduces upgrading a database created by an older
// version (whose incomes table has no external_id column) and verifies Open
// migrates it without error.
func TestMigrateAddsExternalID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// Simulate an old database: incomes table without external_id.
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.Exec(`CREATE TABLE incomes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		description TEXT NOT NULL,
		amount REAL NOT NULL,
		date DATE NOT NULL,
		category TEXT NOT NULL DEFAULT 'Outros',
		confirmed INTEGER NOT NULL DEFAULT 1,
		source TEXT NOT NULL DEFAULT 'manual',
		pokemon_account_id INTEGER,
		period TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatal(err)
	}
	old.Close()

	// Opening with the current code must migrate cleanly.
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open on old db failed: %v", err)
	}
	defer db.Close()

	if !hasColumn(db, "incomes", "external_id") {
		t.Fatal("external_id column was not added by migration")
	}
	// The unique index must exist and enforce uniqueness on non-null values.
	if _, err := db.Exec(`INSERT INTO incomes (description, amount, date, external_id) VALUES ('a', 1, '2026-01-01', 'x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO incomes (description, amount, date, external_id) VALUES ('b', 2, '2026-01-01', 'x')`); err == nil {
		t.Fatal("expected unique-constraint violation on duplicate external_id")
	}
}
