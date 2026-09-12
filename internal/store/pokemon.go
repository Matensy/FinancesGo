package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

const pokemonCols = `id, email, level, team, description, legendaries, shinies, pokemon_count,
	bag_capacity, base_value, ggmax_rate, tags, status, problem_note, problem_at,
	sold_at, sold_value, matures_at, matured, refunded_at, refund_reason, created_at, updated_at`

func scanPokemon(row interface {
	Scan(dest ...any) error
}) (models.PokemonAccount, error) {
	var p models.PokemonAccount
	var team, status string
	var problemAt, soldAt, maturesAt, refundedAt sql.NullString
	var soldValue sql.NullFloat64
	var matured int
	var created, updated string
	err := row.Scan(&p.ID, &p.Email, &p.Level, &team, &p.Description, &p.Legendaries, &p.Shinies,
		&p.PokemonCount, &p.BagCapacity, &p.BaseValue, &p.GGMaxRate, &p.Tags, &status, &p.ProblemNote,
		&problemAt, &soldAt, &soldValue, &maturesAt, &matured, &refundedAt, &p.RefundReason,
		&created, &updated)
	if err != nil {
		return p, err
	}
	p.Team = models.Team(team)
	p.Status = models.PokemonStatus(status)
	p.ProblemAt = nullTime(problemAt)
	p.SoldAt = nullTime(soldAt)
	p.MaturesAt = nullTime(maturesAt)
	p.RefundedAt = nullTime(refundedAt)
	if soldValue.Valid {
		v := soldValue.Float64
		p.SoldValue = &v
	}
	p.Matured = matured == 1
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return p, nil
}

// CreatePokemon inserts a new account and records its initial price snapshot.
func (s *Store) CreatePokemon(p models.PokemonAccount) (int64, error) {
	if p.Status == "" {
		p.Status = models.StatusAtiva
	}
	res, err := s.db.Exec(`INSERT INTO pokemon_accounts
		(email, level, team, description, legendaries, shinies, pokemon_count, bag_capacity,
		 base_value, ggmax_rate, tags, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Email, p.Level, string(p.Team), p.Description, p.Legendaries, p.Shinies, p.PokemonCount,
		p.BagCapacity, p.BaseValue, p.GGMaxRate, p.Tags, string(p.Status), fmtDateTime(time.Now()))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	_ = s.recordPrice(id, p.BaseValue, p.GGMaxRate)
	return id, nil
}

// UpdatePokemon updates the editable fields of an account. If the price changed
// it records a new price-history snapshot.
func (s *Store) UpdatePokemon(p models.PokemonAccount) error {
	prev, err := s.GetPokemon(p.ID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE pokemon_accounts SET
		email=?, level=?, team=?, description=?, legendaries=?, shinies=?, pokemon_count=?,
		bag_capacity=?, base_value=?, ggmax_rate=?, tags=?, updated_at=? WHERE id=?`,
		p.Email, p.Level, string(p.Team), p.Description, p.Legendaries, p.Shinies, p.PokemonCount,
		p.BagCapacity, p.BaseValue, p.GGMaxRate, p.Tags, fmtDateTime(time.Now()), p.ID)
	if err != nil {
		return err
	}
	if prev.BaseValue != p.BaseValue || prev.GGMaxRate != p.GGMaxRate {
		_ = s.recordPrice(p.ID, p.BaseValue, p.GGMaxRate)
	}
	return nil
}

// DeletePokemon removes an account.
func (s *Store) DeletePokemon(id int64) error {
	_, err := s.db.Exec(`DELETE FROM pokemon_accounts WHERE id = ?`, id)
	return err
}

// GetPokemon returns a single account by id.
func (s *Store) GetPokemon(id int64) (models.PokemonAccount, error) {
	row := s.db.QueryRow(`SELECT `+pokemonCols+` FROM pokemon_accounts WHERE id = ?`, id)
	return scanPokemon(row)
}

// ListPokemon returns accounts filtered by status, team and tag (any empty = no filter).
func (s *Store) ListPokemon(status, team, tag string) ([]models.PokemonAccount, error) {
	q := `SELECT ` + pokemonCols + ` FROM pokemon_accounts WHERE 1=1`
	var args []any
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	if team != "" {
		q += ` AND team = ?`
		args = append(args, team)
	}
	if tag != "" {
		q += ` AND (',' || REPLACE(tags, ', ', ',') || ',') LIKE ?`
		args = append(args, "%,"+tag+",%")
	}
	q += ` ORDER BY
		CASE status WHEN 'problema' THEN 0 WHEN 'ativa' THEN 1 WHEN 'anunciada' THEN 2
		WHEN 'vendida' THEN 3 ELSE 4 END, updated_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PokemonAccount
	for rows.Next() {
		p, err := scanPokemon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) recordPrice(accountID int64, base, rate float64) error {
	final := base * (1 + rate)
	_, err := s.db.Exec(`INSERT INTO pokemon_price_history (account_id, base_value, ggmax_rate, final_price, recorded_at)
		VALUES (?, ?, ?, ?, ?)`, accountID, base, rate, final, fmtDateTime(time.Now()))
	return err
}

// PriceHistory returns the price snapshots for an account, newest first.
func (s *Store) PriceHistory(accountID int64) ([]models.PriceHistory, error) {
	rows, err := s.db.Query(`SELECT id, account_id, base_value, ggmax_rate, final_price, recorded_at
		FROM pokemon_price_history WHERE account_id = ? ORDER BY recorded_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PriceHistory
	for rows.Next() {
		var h models.PriceHistory
		var rec string
		if err := rows.Scan(&h.ID, &h.AccountID, &h.BaseValue, &h.GGMaxRate, &h.FinalPrice, &rec); err != nil {
			return nil, err
		}
		h.RecordedAt = parseTime(rec)
		out = append(out, h)
	}
	return out, rows.Err()
}

// ToggleProblem marks an account as having a problem (or clears it, restoring
// it to "ativa"). A note may be attached when flagging.
func (s *Store) ToggleProblem(id int64, note string) error {
	p, err := s.GetPokemon(id)
	if err != nil {
		return err
	}
	now := time.Now()
	if p.Status == models.StatusProblema {
		_, err = s.db.Exec(`UPDATE pokemon_accounts SET status=?, problem_at=NULL, updated_at=? WHERE id=?`,
			string(models.StatusAtiva), fmtDateTime(now), id)
		return err
	}
	_, err = s.db.Exec(`UPDATE pokemon_accounts SET status=?, problem_note=?, problem_at=?, updated_at=? WHERE id=?`,
		string(models.StatusProblema), note, fmtDateTime(now), fmtDateTime(now), id)
	return err
}

// SetStatus sets a simple status (e.g. ativa <-> anunciada).
func (s *Store) SetStatus(id int64, status models.PokemonStatus) error {
	_, err := s.db.Exec(`UPDATE pokemon_accounts SET status=?, updated_at=? WHERE id=?`,
		string(status), fmtDateTime(time.Now()), id)
	return err
}

// MarkSold records a sale. The value does NOT enter the balance yet; it matures
// after holdDays. Returns an error if the account is already sold.
func (s *Store) MarkSold(id int64, value float64, soldAt time.Time, holdDays int) error {
	matures := startOfDay(soldAt).AddDate(0, 0, holdDays)
	_, err := s.db.Exec(`UPDATE pokemon_accounts SET
		status=?, sold_at=?, sold_value=?, matures_at=?, matured=0, updated_at=? WHERE id=?`,
		string(models.StatusVendida), fmtDateTime(soldAt), value, fmtDateTime(matures),
		fmtDateTime(time.Now()), id)
	return err
}

// Refund reverses a sale.
//
//   - If the sale had already matured (its value was credited as an income), the
//     income row is kept and a compensating expense is created, so both the
//     inflow and the reversal stay visible in the ledger (net zero effect).
//   - If the sale had not yet matured, no income exists yet, so nothing needs to
//     be reversed financially.
func (s *Store) Refund(id int64, reason string) error {
	p, err := s.GetPokemon(id)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	// If the sale already matured (money was in the balance), log an outflow so
	// the balance returns to where it was while keeping the history traceable.
	if p.Matured && p.SoldValue != nil {
		desc := fmt.Sprintf("Reembolso conta Pokémon GO #%d", id)
		if strings.TrimSpace(reason) != "" {
			desc += " - " + reason
		}
		if _, err := tx.Exec(`INSERT INTO expenses (description, amount, date, category, source, pokemon_account_id)
			VALUES (?, ?, ?, ?, 'pokemon', ?)`,
			desc, *p.SoldValue, fmtDate(now), "Reembolso Pokémon GO", id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE pokemon_accounts SET
		status=?, refunded_at=?, refund_reason=?, matured=0, updated_at=? WHERE id=?`,
		string(models.StatusReembolsada), fmtDateTime(now), reason, fmtDateTime(now), id); err != nil {
		return err
	}
	return tx.Commit()
}

// MatureSales processes all sold accounts whose maturation date has passed and
// that have not yet been credited. Each becomes a confirmed income. Returns the
// number of accounts matured.
func (s *Store) MatureSales(on time.Time) (int, error) {
	rows, err := s.db.Query(`SELECT `+pokemonCols+` FROM pokemon_accounts
		WHERE status = 'vendida' AND matured = 0 AND matures_at IS NOT NULL AND matures_at <= ?`,
		fmtDateTime(on))
	if err != nil {
		return 0, err
	}
	var due []models.PokemonAccount
	for rows.Next() {
		p, err := scanPokemon(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, p := range due {
		if p.SoldValue == nil {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return count, err
		}
		desc := fmt.Sprintf("Venda conta Pokémon GO #%d", p.ID)
		if p.Email != "" {
			desc = fmt.Sprintf("Venda conta Pokémon GO (%s)", p.Email)
		}
		date := on
		if p.MaturesAt != nil {
			date = *p.MaturesAt
		}
		if _, err := tx.Exec(`INSERT INTO incomes
			(description, amount, date, category, confirmed, source, pokemon_account_id)
			VALUES (?, ?, ?, ?, 1, 'pokemon', ?)`,
			desc, *p.SoldValue, fmtDate(date), "Venda Pokémon GO", p.ID); err != nil {
			tx.Rollback()
			return count, err
		}
		if _, err := tx.Exec(`UPDATE pokemon_accounts SET matured = 1, updated_at = ? WHERE id = ?`,
			fmtDateTime(time.Now()), p.ID); err != nil {
			tx.Rollback()
			return count, err
		}
		if err := tx.Commit(); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// PendingSalesValue returns the total sold value that will mature in (from, until].
func (s *Store) PendingSalesValue(from, until time.Time) (float64, error) {
	var total sql.NullFloat64
	err := s.db.QueryRow(`SELECT SUM(sold_value) FROM pokemon_accounts
		WHERE status = 'vendida' AND matured = 0 AND matures_at > ? AND matures_at <= ?`,
		fmtDateTime(from), fmtDateTime(until)).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// PokemonStats holds catalog summary counts for the dashboard.
type PokemonStats struct {
	Total          int
	Ativa          int
	Anunciada      int
	Vendida        int
	Reembolsada    int
	Problema       int
	PendingMature  int
	CatalogValue   float64 // final price of active/announced accounts
	PendingValue   float64 // sold, not yet matured
	SoldMonthValue float64 // matured incomes this month
}

// Stats aggregates account counts and values.
func (s *Store) PokemonStats(year int, month time.Month) (PokemonStats, error) {
	var st PokemonStats
	all, err := s.ListPokemon("", "", "")
	if err != nil {
		return st, err
	}
	for _, p := range all {
		st.Total++
		switch p.Status {
		case models.StatusAtiva:
			st.Ativa++
			st.CatalogValue += p.FinalPrice()
		case models.StatusAnunciada:
			st.Anunciada++
			st.CatalogValue += p.FinalPrice()
		case models.StatusVendida:
			st.Vendida++
			if !p.Matured && p.SoldValue != nil {
				st.PendingMature++
				st.PendingValue += *p.SoldValue
			}
		case models.StatusReembolsada:
			st.Reembolsada++
		case models.StatusProblema:
			st.Problema++
		}
	}
	var sold sql.NullFloat64
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	err = s.db.QueryRow(`SELECT SUM(amount) FROM incomes
		WHERE source='pokemon' AND category='Venda Pokémon GO' AND date >= ? AND date < ?`,
		fmtDate(start), fmtDate(end)).Scan(&sold)
	if err != nil {
		return st, err
	}
	st.SoldMonthValue = sold.Float64
	return st, nil
}

// AllTags returns the distinct set of tags used across accounts.
func (s *Store) AllTags() ([]string, error) {
	rows, err := s.db.Query(`SELECT tags FROM pokemon_accounts WHERE tags <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			return nil, err
		}
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out, rows.Err()
}
