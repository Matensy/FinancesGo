// Package backup handles SQLite database backup and restore.
package backup

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// Manager performs online backups of the SQLite database.
type Manager struct {
	db        *sql.DB
	dbPath    string
	backupDir string
}

// New returns a backup Manager.
func New(db *sql.DB, dbPath, backupDir string) *Manager {
	return &Manager{db: db, dbPath: dbPath, backupDir: backupDir}
}

// snapshot writes a consistent copy of the database to dst using SQLite's
// VACUUM INTO, which is safe while the app is running.
func (m *Manager) snapshot(dst string) error {
	if _, err := m.db.Exec(`VACUUM INTO ?`, dst); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	return nil
}

// Create makes a timestamped backup in the backup directory and returns its path.
func (m *Manager) Create() (string, error) {
	name := fmt.Sprintf("backup-%s.db", time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(m.backupDir, name)
	if err := m.snapshot(path); err != nil {
		return "", err
	}
	return path, nil
}

// ExportTo writes a fresh snapshot to an arbitrary destination path.
func (m *Manager) ExportTo(dst string) error {
	if dst == "" {
		return fmt.Errorf("destino vazio")
	}
	// VACUUM INTO fails if the file exists.
	_ = os.Remove(dst)
	return m.snapshot(dst)
}

// SnapshotToTemp writes a snapshot to a temp file and returns its path. The
// caller is responsible for removing it.
func (m *Manager) SnapshotToTemp() (string, error) {
	f, err := os.CreateTemp("", "financesgo-*.db")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	_ = os.Remove(path)
	if err := m.snapshot(path); err != nil {
		return "", err
	}
	return path, nil
}

// List returns backup file names, newest first.
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

// Prune keeps only the newest keep backups.
func (m *Manager) Prune(keep int) error {
	names, err := m.List()
	if err != nil {
		return err
	}
	for i := keep; i < len(names); i++ {
		_ = os.Remove(filepath.Join(m.backupDir, names[i]))
	}
	return nil
}

// Replace atomically replaces the live database file with the uploaded one.
// The database connection must be closed first; the caller then restarts the app.
func Replace(srcReader io.Reader, dbPath string) error {
	tmp := dbPath + ".import.tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, srcReader); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// Validate that the uploaded file is a usable SQLite database.
	if err := validateSQLite(tmp); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("arquivo inválido: %w", err)
	}
	// Remove WAL/SHM sidecars from the old DB so they don't clobber the import.
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	if err := os.Rename(tmp, dbPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func validateSQLite(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check: %s", result)
	}
	return nil
}
