package store

import (
	"database/sql"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

// Earnings are logged manually by the user (independent from GGMAX/finance) and
// drive the daily goal tracker.

// AddEarning records a manual earning entry.
func (s *Store) AddEarning(amount float64, date time.Time, note string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO earnings_log (date, amount, note) VALUES (?, ?, ?)`,
		fmtDate(startOfDay(date)), amount, note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteEarning removes a manual earning entry.
func (s *Store) DeleteEarning(id int64) error {
	_, err := s.db.Exec(`DELETE FROM earnings_log WHERE id = ?`, id)
	return err
}

// EarningsBetween sums manual earnings in [from, to] (inclusive, by day).
func (s *Store) EarningsBetween(from, to time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM earnings_log WHERE date >= ? AND date <= ?`,
		fmtDate(startOfDay(from)), fmtDate(startOfDay(to))).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// FirstEarningDay returns the earliest earning day on/after "from", if any.
func (s *Store) FirstEarningDay(from time.Time) (time.Time, bool, error) {
	var d sql.NullString
	err := s.db.QueryRow(`SELECT MIN(date) FROM earnings_log WHERE date >= ?`,
		fmtDate(startOfDay(from))).Scan(&d)
	if err != nil {
		return time.Time{}, false, err
	}
	if !d.Valid || d.String == "" {
		return time.Time{}, false, nil
	}
	t := parseTime(d.String)
	if t.IsZero() {
		return time.Time{}, false, nil
	}
	return startOfDay(t), true, nil
}

// DayEarning is one day's total earnings.
type DayEarning struct {
	Date   time.Time
	Amount float64
}

// EarningsByDay returns the total earnings for each of the last n days (oldest first).
func (s *Store) EarningsByDay(n int) ([]DayEarning, error) {
	today := startOfDay(time.Now())
	from := today.AddDate(0, 0, -(n - 1))
	rows, err := s.db.Query(`SELECT date, SUM(amount) FROM earnings_log
		WHERE date >= ? AND date <= ? GROUP BY date`, fmtDate(from), fmtDate(today))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDay := map[string]float64{}
	for rows.Next() {
		var d string
		var amt float64
		if err := rows.Scan(&d, &amt); err != nil {
			return nil, err
		}
		byDay[parseTime(d).Format(dateLayout)] = amt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]DayEarning, 0, n)
	for i := 0; i < n; i++ {
		day := from.AddDate(0, 0, i)
		out = append(out, DayEarning{Date: day, Amount: byDay[day.Format(dateLayout)]})
	}
	return out, nil
}

// ListRecentEarnings returns the latest manual earning entries.
func (s *Store) ListRecentEarnings(limit int) ([]models.Earning, error) {
	rows, err := s.db.Query(`SELECT id, date, amount, note, created_at FROM earnings_log
		ORDER BY date DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Earning
	for rows.Next() {
		var e models.Earning
		var date, created string
		if err := rows.Scan(&e.ID, &date, &e.Amount, &e.Note, &created); err != nil {
			return nil, err
		}
		e.Date = parseTime(date)
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}
