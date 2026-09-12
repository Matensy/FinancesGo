package store

import (
	"database/sql"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

// --- One-off / concrete incomes ---

// CreateIncome inserts a concrete income entry.
func (s *Store) CreateIncome(in models.Income) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO incomes
		(description, amount, date, category, confirmed, source, pokemon_account_id, period)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Description, in.Amount, fmtDate(in.Date), in.Category,
		boolToInt(in.Confirmed), sourceOr(in.Source), in.PokemonID, in.Period)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateIncome updates an income entry.
func (s *Store) UpdateIncome(in models.Income) error {
	_, err := s.db.Exec(`UPDATE incomes SET description=?, amount=?, date=?, category=?, confirmed=? WHERE id=?`,
		in.Description, in.Amount, fmtDate(in.Date), in.Category, boolToInt(in.Confirmed), in.ID)
	return err
}

// DeleteIncome removes an income entry.
func (s *Store) DeleteIncome(id int64) error {
	_, err := s.db.Exec(`DELETE FROM incomes WHERE id = ?`, id)
	return err
}

func scanIncomes(rows *sql.Rows) ([]models.Income, error) {
	defer rows.Close()
	var out []models.Income
	for rows.Next() {
		var in models.Income
		var date, created string
		var confirmed int
		var pokemonID sql.NullInt64
		var period sql.NullString
		if err := rows.Scan(&in.ID, &in.Description, &in.Amount, &date, &in.Category,
			&confirmed, &in.Source, &pokemonID, &period, &created); err != nil {
			return nil, err
		}
		in.Date = parseTime(date)
		in.Confirmed = confirmed == 1
		if pokemonID.Valid {
			id := pokemonID.Int64
			in.PokemonID = &id
		}
		if period.Valid {
			p := period.String
			in.Period = &p
		}
		in.CreatedAt = parseTime(created)
		out = append(out, in)
	}
	return out, rows.Err()
}

const incomeCols = `id, description, amount, date, category, confirmed, source, pokemon_account_id, period, created_at`

// ListIncomesForMonth returns incomes dated within the given month.
func (s *Store) ListIncomesForMonth(year int, month time.Month, category string) ([]models.Income, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	q := `SELECT ` + incomeCols + ` FROM incomes WHERE date >= ? AND date < ?`
	args := []any{fmtDate(start), fmtDate(end)}
	if category != "" {
		q += ` AND category = ?`
		args = append(args, category)
	}
	q += ` ORDER BY date DESC, id DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	return scanIncomes(rows)
}

// ConfirmedIncomeUpTo returns the sum of confirmed incomes dated on/before day.
func (s *Store) ConfirmedIncomeUpTo(day time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM incomes WHERE confirmed = 1 AND date <= ?`,
		fmtDate(startOfDay(day))).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// FutureConfirmedIncome returns the sum of confirmed incomes with date in (from, until].
func (s *Store) FutureConfirmedIncome(from, until time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM incomes WHERE confirmed = 1 AND date > ? AND date <= ?`,
		fmtDate(startOfDay(from)), fmtDate(startOfDay(until))).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// --- Recurring incomes ---

// CreateRecurringIncome inserts a recurring income template.
func (s *Store) CreateRecurringIncome(r models.RecurringIncome) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO recurring_incomes (name, amount, day, category, is_salary, active)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.Name, r.Amount, r.Day, r.Category, boolToInt(r.IsSalary), boolToInt(r.Active))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateRecurringIncome updates a recurring income template.
func (s *Store) UpdateRecurringIncome(r models.RecurringIncome) error {
	_, err := s.db.Exec(`UPDATE recurring_incomes SET name=?, amount=?, day=?, category=?, is_salary=?, active=? WHERE id=?`,
		r.Name, r.Amount, r.Day, r.Category, boolToInt(r.IsSalary), boolToInt(r.Active), r.ID)
	return err
}

// DeleteRecurringIncome removes a recurring income template.
func (s *Store) DeleteRecurringIncome(id int64) error {
	_, err := s.db.Exec(`DELETE FROM recurring_incomes WHERE id = ?`, id)
	return err
}

// ListRecurringIncomes returns all recurring income templates.
func (s *Store) ListRecurringIncomes() ([]models.RecurringIncome, error) {
	rows, err := s.db.Query(`SELECT id, name, amount, day, category, is_salary, active, created_at
		FROM recurring_incomes ORDER BY day, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.RecurringIncome
	for rows.Next() {
		var r models.RecurringIncome
		var isSalary, active int
		var created string
		if err := rows.Scan(&r.ID, &r.Name, &r.Amount, &r.Day, &r.Category, &isSalary, &active, &created); err != nil {
			return nil, err
		}
		r.IsSalary = isSalary == 1
		r.Active = active == 1
		r.CreatedAt = parseTime(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// recurringMaterialized reports whether a recurring income was already
// materialized for a given period.
func (s *Store) recurringMaterialized(name, period string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM incomes WHERE source = 'recurring' AND description = ? AND period = ?`,
		name, period).Scan(&n)
	return n > 0, err
}

// MaterializeRecurringIncomes creates concrete income entries for any recurring
// income whose day has arrived (on/before "on") in the current month and that
// has not yet been materialized. Returns the number created.
func (s *Store) MaterializeRecurringIncomes(on time.Time) (int, error) {
	recs, err := s.ListRecurringIncomes()
	if err != nil {
		return 0, err
	}
	on = startOfDay(on)
	period := periodKey(on.Year(), on.Month())
	created := 0
	for _, r := range recs {
		if !r.Active {
			continue
		}
		due := dueDateFor(on.Year(), on.Month(), r.Day)
		if due.After(on) {
			continue // not due yet this month
		}
		done, err := s.recurringMaterialized(r.Name, period)
		if err != nil {
			return created, err
		}
		if done {
			continue
		}
		p := period
		if _, err := s.CreateIncome(models.Income{
			Description: r.Name,
			Amount:      r.Amount,
			Date:        due,
			Category:    r.Category,
			Confirmed:   true,
			Source:      "recurring",
			Period:      &p,
		}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// FutureRecurringIncome estimates recurring income occurrences in (from, until].
func (s *Store) FutureRecurringIncome(from, until time.Time) (float64, error) {
	recs, err := s.ListRecurringIncomes()
	if err != nil {
		return 0, err
	}
	from = startOfDay(from)
	until = startOfDay(until)
	var total float64
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.Local)
	end := time.Date(until.Year(), until.Month(), 1, 0, 0, 0, 0, time.Local)
	for !cur.After(end) {
		period := periodKey(cur.Year(), cur.Month())
		for _, r := range recs {
			if !r.Active {
				continue
			}
			occ := dueDateFor(cur.Year(), cur.Month(), r.Day)
			if !occ.After(from) || occ.After(until) {
				continue
			}
			if done, _ := s.recurringMaterialized(r.Name, period); done {
				continue
			}
			total += r.Amount
		}
		cur = cur.AddDate(0, 1, 0)
	}
	return total, nil
}

// NextSalaryDate returns the next occurrence (>= today) of a salary income.
func (s *Store) NextSalaryDate(from time.Time) (time.Time, bool, error) {
	recs, err := s.ListRecurringIncomes()
	if err != nil {
		return time.Time{}, false, err
	}
	from = startOfDay(from)
	var best time.Time
	found := false
	for _, r := range recs {
		if !r.Active || !r.IsSalary {
			continue
		}
		for i := 0; i < 2; i++ { // check this month and next
			m := from.AddDate(0, i, 0)
			occ := dueDateFor(m.Year(), m.Month(), r.Day)
			if occ.Before(from) {
				continue
			}
			if !found || occ.Before(best) {
				best = occ
				found = true
			}
			break
		}
	}
	return best, found, nil
}

func sourceOr(s string) string {
	if s == "" {
		return "manual"
	}
	return s
}
