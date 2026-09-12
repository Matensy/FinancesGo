package store

import (
	"database/sql"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

// CreateExpense inserts a one-off expense.
func (s *Store) CreateExpense(e models.Expense) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO expenses (description, amount, date, category, source, pokemon_account_id)
		VALUES (?, ?, ?, ?, ?, ?)`,
		e.Description, e.Amount, fmtDate(e.Date), e.Category, sourceOr(e.Source), e.PokemonID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateExpense updates an expense.
func (s *Store) UpdateExpense(e models.Expense) error {
	_, err := s.db.Exec(`UPDATE expenses SET description=?, amount=?, date=?, category=? WHERE id=?`,
		e.Description, e.Amount, fmtDate(e.Date), e.Category, e.ID)
	return err
}

// DeleteExpense removes an expense.
func (s *Store) DeleteExpense(id int64) error {
	_, err := s.db.Exec(`DELETE FROM expenses WHERE id = ?`, id)
	return err
}

func scanExpenses(rows *sql.Rows) ([]models.Expense, error) {
	defer rows.Close()
	var out []models.Expense
	for rows.Next() {
		var e models.Expense
		var date, created string
		var pokemonID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Description, &e.Amount, &date, &e.Category,
			&e.Source, &pokemonID, &created); err != nil {
			return nil, err
		}
		e.Date = parseTime(date)
		if pokemonID.Valid {
			id := pokemonID.Int64
			e.PokemonID = &id
		}
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

const expenseCols = `id, description, amount, date, category, source, pokemon_account_id, created_at`

// ListExpensesForMonth returns expenses dated within the given month.
func (s *Store) ListExpensesForMonth(year int, month time.Month, category string) ([]models.Expense, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	q := `SELECT ` + expenseCols + ` FROM expenses WHERE date >= ? AND date < ?`
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
	return scanExpenses(rows)
}

// ExpensesUpTo returns the sum of expenses dated on/before day.
func (s *Store) ExpensesUpTo(day time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM expenses WHERE date <= ?`,
		fmtDate(startOfDay(day))).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// PaidBillsTotal returns the sum of all bill payments (money already spent).
func (s *Store) PaidBillsTotal() (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM bill_payments`).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// PaidBillsForMonth returns the sum of bill payments in a given period.
func (s *Store) PaidBillsForMonth(year int, month time.Month) (float64, error) {
	var total sql.NullFloat64
	period := periodKey(year, month)
	err := s.db.QueryRow(`SELECT SUM(amount) FROM bill_payments WHERE period = ?`, period).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}
