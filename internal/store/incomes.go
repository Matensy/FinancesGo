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
		(description, amount, date, category, confirmed, source, pokemon_account_id, period, external_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Description, in.Amount, fmtDate(in.Date), in.Category,
		boolToInt(in.Confirmed), sourceOr(in.Source), in.PokemonID, in.Period, nullStr(in.ExternalID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
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
		var confirmed, voided int
		var pokemonID sql.NullInt64
		var period, extID sql.NullString
		if err := rows.Scan(&in.ID, &in.Description, &in.Amount, &date, &in.Category,
			&confirmed, &in.Source, &pokemonID, &period, &extID, &voided, &created); err != nil {
			return nil, err
		}
		in.Date = parseTime(date)
		in.Confirmed = confirmed == 1
		in.Voided = voided == 1
		if pokemonID.Valid {
			id := pokemonID.Int64
			in.PokemonID = &id
		}
		if period.Valid {
			p := period.String
			in.Period = &p
		}
		if extID.Valid {
			in.ExternalID = extID.String
		}
		in.CreatedAt = parseTime(created)
		out = append(out, in)
	}
	return out, rows.Err()
}

const incomeCols = `id, description, amount, date, category, confirmed, source, pokemon_account_id, period, external_id, voided, created_at`

// bankSources is the SQL fragment excluding money that lives on the GGMAX
// platform (raw imported sales) from the real bank balance. Withdrawals
// (ggmax_withdraw) DO count, because that money reached the bank.
const notGGMAX = ` AND voided = 0 AND source <> 'ggmax'`

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

// ListBankIncomesForMonth returns incomes for the month excluding GGMAX
// platform sales (which are shown in the GGMAX wallet, not the bank).
func (s *Store) ListBankIncomesForMonth(year int, month time.Month, category string) ([]models.Income, error) {
	all, err := s.ListIncomesForMonth(year, month, category)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, in := range all {
		if in.Source == "ggmax" {
			continue
		}
		out = append(out, in)
	}
	return out, nil
}

// ConfirmedIncomeUpTo returns the sum of confirmed bank incomes dated on/before
// day (GGMAX platform sales excluded; withdrawals included).
func (s *Store) ConfirmedIncomeUpTo(day time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM incomes WHERE confirmed = 1 AND date <= ?`+notGGMAX,
		fmtDate(startOfDay(day))).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// FutureConfirmedIncome returns the sum of confirmed bank incomes with date in
// (from, until]. GGMAX platform sales are excluded — that money only reaches the
// bank through a withdrawal.
func (s *Store) FutureConfirmedIncome(from, until time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(amount) FROM incomes WHERE confirmed = 1 AND date > ? AND date <= ?`+notGGMAX,
		fmtDate(startOfDay(from)), fmtDate(startOfDay(until))).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// SetIncomeVoided marks an income as voided (refunded) or restores it.
func (s *Store) SetIncomeVoided(id int64, voided bool) error {
	_, err := s.db.Exec(`UPDATE incomes SET voided = ? WHERE id = ?`, boolToInt(voided), id)
	return err
}

// GGMAXWallet holds the money currently held on the GGMAX platform.
type GGMAXWallet struct {
	Available float64 // released, not yet withdrawn to the bank (with manual adjustment)
	Pending   float64 // "a liberar": not released yet (with manual adjustment)
	Withdrawn float64 // total already moved to the bank
	Refunded  float64 // total voided (refunds/problems)

	AdjAvailable  float64 // manual offset applied to Available
	AdjPending    float64 // manual offset applied to Pending
	BaseAvailable float64 // Available before the manual adjustment
	BasePending   float64 // Pending before the manual adjustment
}

// GGMAXWalletState computes the current GGMAX balances as of "on".
func (s *Store) GGMAXWalletState(on time.Time) (GGMAXWallet, error) {
	var w GGMAXWallet
	today := fmtDate(startOfDay(on))

	scan := func(q string, args ...any) (float64, error) {
		var v sql.NullFloat64
		if err := s.db.QueryRow(q, args...).Scan(&v); err != nil {
			return 0, err
		}
		return v.Float64, nil
	}
	released, err := scan(`SELECT SUM(amount) FROM incomes WHERE source='ggmax' AND voided=0 AND date <= ?`, today)
	if err != nil {
		return w, err
	}
	pending, err := scan(`SELECT SUM(amount) FROM incomes WHERE source='ggmax' AND voided=0 AND date > ?`, today)
	if err != nil {
		return w, err
	}
	withdrawn, err := scan(`SELECT SUM(amount) FROM incomes WHERE source='ggmax_withdraw' AND voided=0`)
	if err != nil {
		return w, err
	}
	refunded, err := scan(`SELECT SUM(amount) FROM incomes WHERE source='ggmax' AND voided=1`)
	if err != nil {
		return w, err
	}
	w.Withdrawn = withdrawn
	w.Refunded = refunded
	// Base may go negative when withdrawals exceed imported released sales
	// (e.g. the user relies on a manual adjustment); only the final displayed
	// Available is clamped, so withdrawals always reduce the shown balance.
	w.BaseAvailable = released - withdrawn
	w.BasePending = pending

	adjA, adjP := s.GGMAXAdjustments()
	w.AdjAvailable = adjA
	w.AdjPending = adjP
	w.Available = w.BaseAvailable + adjA
	if w.Available < 0 {
		w.Available = 0
	}
	w.Pending = w.BasePending + adjP
	if w.Pending < 0 {
		w.Pending = 0
	}
	return w, nil
}

// ListGGMAXSales returns imported GGMAX sales (source='ggmax'), newest first.
func (s *Store) ListGGMAXSales() ([]models.Income, error) {
	rows, err := s.db.Query(`SELECT ` + incomeCols + ` FROM incomes WHERE source='ggmax' ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	return scanIncomes(rows)
}

// RegisterGGMAXWithdrawal records money moved from GGMAX to the bank: it counts
// in the bank balance and reduces the GGMAX available balance.
func (s *Store) RegisterGGMAXWithdrawal(amount float64, date time.Time) (int64, error) {
	return s.CreateIncome(models.Income{
		Description: "Saque GGMAX",
		Amount:      amount,
		Date:        date,
		Category:    "Saque GGMAX",
		Confirmed:   true,
		Source:      "ggmax_withdraw",
	})
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
