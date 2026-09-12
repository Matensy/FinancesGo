// Package store provides data-access methods over the SQLite database.
package store

import (
	"database/sql"
	"strings"
	"time"
)

// Store wraps the database connection and exposes typed queries.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB exposes the underlying handle (used by backup routines).
func (s *Store) DB() *sql.DB { return s.db }

const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02 15:04:05"
)

// parseTime parses a timestamp/date string coming from SQLite, trying the
// formats the driver and CURRENT_TIMESTAMP may produce.
func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		dateTimeLayout,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		dateLayout,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Local()
		}
	}
	return time.Time{}
}

func nullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || strings.TrimSpace(ns.String) == "" {
		return nil
	}
	t := parseTime(ns.String)
	if t.IsZero() {
		return nil
	}
	return &t
}

func fmtDate(t time.Time) string     { return t.Format(dateLayout) }
func fmtDateTime(t time.Time) string { return t.Format(dateTimeLayout) }

// startOfDay truncates t to midnight in local time.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
