package store

import (
	"database/sql"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

// MonthFlow holds aggregated inflow/outflow for one month.
type MonthFlow struct {
	Period  string // "2026-09"
	Label   string // "set/26"
	Inflow  float64
	Outflow float64
	Net     float64
}

var monthAbbrPT = []string{"", "jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

// MonthLabel formats a time as "set/26" (Portuguese short month + 2-digit year).
func MonthLabel(t time.Time) string {
	return monthAbbrPT[int(t.Month())] + t.Format("/06")
}

// CashFlow returns inflow/outflow totals for the last n months (oldest first).
func (s *Store) CashFlow(n int) ([]MonthFlow, error) {
	now := time.Now()
	out := make([]MonthFlow, 0, n)
	for i := n - 1; i >= 0; i-- {
		m := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -i, 0)
		start := m
		end := m.AddDate(0, 1, 0)

		var inflow, billOut, expOut sql.NullFloat64
		if err := s.db.QueryRow(`SELECT SUM(amount) FROM incomes WHERE confirmed=1 AND date >= ? AND date < ?`,
			fmtDate(start), fmtDate(end)).Scan(&inflow); err != nil {
			return nil, err
		}
		if err := s.db.QueryRow(`SELECT SUM(amount) FROM bill_payments WHERE period = ?`,
			periodKey(m.Year(), m.Month())).Scan(&billOut); err != nil {
			return nil, err
		}
		if err := s.db.QueryRow(`SELECT SUM(amount) FROM expenses WHERE date >= ? AND date < ?`,
			fmtDate(start), fmtDate(end)).Scan(&expOut); err != nil {
			return nil, err
		}
		mf := MonthFlow{
			Period:  periodKey(m.Year(), m.Month()),
			Label:   monthAbbrPT[int(m.Month())] + m.Format("/06"),
			Inflow:  inflow.Float64,
			Outflow: billOut.Float64 + expOut.Float64,
		}
		mf.Net = mf.Inflow - mf.Outflow
		out = append(out, mf)
	}
	return out, nil
}

// CategoryTotal holds a category label with an aggregated amount.
type CategoryTotal struct {
	Category string
	Amount   float64
}

// ExpenseByCategory returns expense + paid-bill totals grouped by category for a month.
func (s *Store) ExpenseByCategory(year int, month time.Month) ([]CategoryTotal, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	totals := map[string]float64{}
	rows, err := s.db.Query(`SELECT category, SUM(amount) FROM expenses
		WHERE date >= ? AND date < ? GROUP BY category`, fmtDate(start), fmtDate(end))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c string
		var a float64
		if err := rows.Scan(&c, &a); err != nil {
			rows.Close()
			return nil, err
		}
		totals[c] += a
	}
	rows.Close()

	// Paid bills grouped by the bill's category.
	brows, err := s.db.Query(`SELECT b.category, SUM(bp.amount) FROM bill_payments bp
		JOIN bills b ON b.id = bp.bill_id WHERE bp.period = ? GROUP BY b.category`,
		periodKey(year, month))
	if err != nil {
		return nil, err
	}
	for brows.Next() {
		var c string
		var a float64
		if err := brows.Scan(&c, &a); err != nil {
			brows.Close()
			return nil, err
		}
		totals[c] += a
	}
	brows.Close()

	out := make([]CategoryTotal, 0, len(totals))
	for c, a := range totals {
		out = append(out, CategoryTotal{Category: c, Amount: a})
	}
	return out, nil
}

// AllCategories returns the distinct set of categories used across bills,
// incomes and expenses (for filter dropdowns).
func (s *Store) AllCategories() ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(q string) error {
		rows, err := s.db.Query(q)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				return err
			}
			if c != "" && !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
		return rows.Err()
	}
	for _, q := range []string{
		`SELECT DISTINCT category FROM bills`,
		`SELECT DISTINCT category FROM incomes`,
		`SELECT DISTINCT category FROM expenses`,
	} {
		if err := add(q); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// --- Export history ---

// AddExportRecord stores an export-history entry.
func (s *Store) AddExportRecord(r models.ExportRecord) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO export_history (filename, format, path, period, account_count)
		VALUES (?, ?, ?, ?, ?)`, r.Filename, r.Format, r.Path, r.Period, r.AccountCount)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListExportRecords returns the export history, newest first.
func (s *Store) ListExportRecords() ([]models.ExportRecord, error) {
	rows, err := s.db.Query(`SELECT id, filename, format, path, period, account_count, created_at
		FROM export_history ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ExportRecord
	for rows.Next() {
		var r models.ExportRecord
		var created string
		if err := rows.Scan(&r.ID, &r.Filename, &r.Format, &r.Path, &r.Period, &r.AccountCount, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = parseTime(created)
		out = append(out, r)
	}
	return out, rows.Err()
}
