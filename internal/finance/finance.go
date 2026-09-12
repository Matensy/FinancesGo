// Package finance computes the current balance and the spending projection.
package finance

import (
	"math"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
	"github.com/Matensy/FinancesGo/internal/store"
)

// Engine derives balance and projection figures from the store.
type Engine struct {
	store *store.Store
}

// New returns a finance Engine.
func New(s *store.Store) *Engine { return &Engine{store: s} }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Balance returns the current available balance:
//
//	initial + confirmed incomes (<= today) - expenses (<= today) - paid bills
func (e *Engine) Balance() (float64, error) {
	cfg, err := e.store.Settings()
	if err != nil {
		return 0, err
	}
	today := time.Now()
	income, err := e.store.ConfirmedIncomeUpTo(today)
	if err != nil {
		return 0, err
	}
	expenses, err := e.store.ExpensesUpTo(today)
	if err != nil {
		return 0, err
	}
	paidBills, err := e.store.PaidBillsTotal()
	if err != nil {
		return 0, err
	}
	return round2(cfg.InitialBalance + income - expenses - paidBills), nil
}

// available returns the projected spendable amount at day "until" (inclusive):
//
//	balance + incoming (future confirmed + recurring + maturing sales) - unpaid bills due
func (e *Engine) available(balance float64, from, until time.Time) (models.ProjectionPoint, error) {
	from = startOfDay(from)
	until = startOfDay(until)

	futureIncome, err := e.store.FutureConfirmedIncome(from, until)
	if err != nil {
		return models.ProjectionPoint{}, err
	}
	recurring, err := e.store.FutureRecurringIncome(from, until)
	if err != nil {
		return models.ProjectionPoint{}, err
	}
	// Pending Pokémon sales maturing within the window (end of "until" day).
	maturing, err := e.store.PendingSalesValue(from, until.Add(24*time.Hour-time.Second))
	if err != nil {
		return models.ProjectionPoint{}, err
	}
	billsDue, _, err := e.store.UnpaidBillsDueBy(from.AddDate(0, 0, -3650), until) // include overdue
	if err != nil {
		return models.ProjectionPoint{}, err
	}
	incoming := futureIncome + recurring + maturing
	return models.ProjectionPoint{
		Date:          until,
		Available:     round2(balance + incoming - billsDue),
		IncomingTotal: round2(incoming),
		BillsDue:      round2(billsDue),
	}, nil
}

// Projection builds the full spending timeline.
func (e *Engine) Projection() (models.Projection, error) {
	balance, err := e.Balance()
	if err != nil {
		return models.Projection{}, err
	}
	today := startOfDay(time.Now())

	var proj models.Projection
	proj.Balance = balance
	proj.Generated = time.Now()

	type spec struct {
		label string
		until time.Time
	}
	specs := []spec{
		{"Hoje", today},
		{"Amanhã", today.AddDate(0, 0, 1)},
		{"Esta semana", today.AddDate(0, 0, 7)},
		{"Próximos 30 dias", today.AddDate(0, 0, 30)},
	}
	if sal, ok, _ := e.store.NextSalaryDate(today); ok && sal.After(today) {
		specs = append(specs, spec{"Até o próximo salário", sal})
	}

	for _, sp := range specs {
		p, err := e.available(balance, today, sp.until)
		if err != nil {
			return proj, err
		}
		p.Label = sp.label
		proj.Points = append(proj.Points, p)
	}
	if len(proj.Points) > 0 {
		proj.Today = proj.Points[0]
	}
	return proj, nil
}

// SimulateSale returns the balance impact if an account with the given final
// price were to sell and mature now: the current balance plus that value.
func (e *Engine) SimulateSale(finalPrice float64) (current, projected float64, err error) {
	current, err = e.Balance()
	if err != nil {
		return 0, 0, err
	}
	return current, round2(current + finalPrice), nil
}
