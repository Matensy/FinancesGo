package store

import (
	"database/sql"
	"time"
)

// earnings are GGMAX sales counted by the day they were sold (sale_date), not
// when the money is released. Voided (refunded) sales don't count. Rows without
// a sale_date fall back to their income date.
const earningsWhere = `source = 'ggmax' AND voided = 0`

func saleDayExpr() string { return `COALESCE(sale_date, date)` }

// EarningsBetween sums sales made in [from, to] (inclusive, by sale day).
func (s *Store) EarningsBetween(from, to time.Time) (float64, error) {
	var total sql.NullFloat64
	q := `SELECT SUM(amount) FROM incomes WHERE ` + earningsWhere +
		` AND ` + saleDayExpr() + ` >= ? AND ` + saleDayExpr() + ` <= ?`
	if err := s.db.QueryRow(q, fmtDate(startOfDay(from)), fmtDate(startOfDay(to))).Scan(&total); err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// FirstEarningDay returns the earliest sale day on/after "from", if any.
func (s *Store) FirstEarningDay(from time.Time) (time.Time, bool, error) {
	var d sql.NullString
	q := `SELECT MIN(` + saleDayExpr() + `) FROM incomes WHERE ` + earningsWhere +
		` AND ` + saleDayExpr() + ` >= ?`
	if err := s.db.QueryRow(q, fmtDate(startOfDay(from))).Scan(&d); err != nil {
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

// DayEarning is one day's total sales.
type DayEarning struct {
	Date   time.Time
	Amount float64
}

// EarningsByDay returns the total sales for each of the last n days (oldest first).
func (s *Store) EarningsByDay(n int) ([]DayEarning, error) {
	today := startOfDay(time.Now())
	from := today.AddDate(0, 0, -(n - 1))
	rows, err := s.db.Query(`SELECT `+saleDayExpr()+` AS d, SUM(amount) FROM incomes
		WHERE `+earningsWhere+` AND `+saleDayExpr()+` >= ? AND `+saleDayExpr()+` <= ?
		GROUP BY d`, fmtDate(from), fmtDate(today))
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
