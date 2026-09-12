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

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
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
	// Count unpaid bills that fall due from the start of the current month up to
	// the target day. This includes bills already overdue this month, without
	// letting recurring bills accumulate endlessly across past months.
	billsDue, _, err := e.store.UnpaidBillsDueBy(startOfMonth(from), until)
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

// Recommendation holds the "how much can I spend / am I on track" figures.
type Recommendation struct {
	Balance       float64
	Goal          float64
	DaysLeft      int
	IncomeToCome  float64 // confirmed bank income still to arrive this month
	BillsToCome   float64 // unpaid bills due through end of month
	CardDue       float64 // amount owed on the credit card
	ProjectedEnd  float64 // projected balance at month end after obligations
	FreeThisMonth float64 // discretionary amount and still hit the goal
	DailyBudget   float64 // FreeThisMonth / DaysLeft
	InRed         bool    // projected end below zero
	RedAmount     float64
	BelowGoal     bool    // projected end below the goal
	GoalGap       float64 // how much is missing to reach the goal
	HasGoal       bool
}

func lastDayOfMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// Recommend computes spending recommendations for the current month.
func (e *Engine) Recommend() (Recommendation, error) {
	var rec Recommendation
	cfg, err := e.store.Settings()
	if err != nil {
		return rec, err
	}
	balance, err := e.Balance()
	if err != nil {
		return rec, err
	}
	today := startOfDay(time.Now())
	monthEnd := time.Date(today.Year(), today.Month(), lastDayOfMonth(today), 0, 0, 0, 0, today.Location())

	income, err := e.store.FutureConfirmedIncome(today, monthEnd)
	if err != nil {
		return rec, err
	}
	bills, _, err := e.store.UnpaidBillsDueBy(startOfMonth(today), monthEnd)
	if err != nil {
		return rec, err
	}

	rec.Balance = balance
	rec.Goal = cfg.MonthlyGoal
	rec.HasGoal = cfg.MonthlyGoal > 0
	rec.DaysLeft = lastDayOfMonth(today) - today.Day() + 1
	if rec.DaysLeft < 1 {
		rec.DaysLeft = 1
	}
	rec.IncomeToCome = round2(income)
	rec.BillsToCome = round2(bills)
	rec.CardDue = round2(cfg.CardUsed)

	rec.ProjectedEnd = round2(balance + income - bills - cfg.CardUsed)
	rec.FreeThisMonth = round2(rec.ProjectedEnd - cfg.MonthlyGoal)
	rec.DailyBudget = round2(rec.FreeThisMonth / float64(rec.DaysLeft))
	if rec.DailyBudget < 0 {
		rec.DailyBudget = 0
	}
	if rec.ProjectedEnd < 0 {
		rec.InRed = true
		rec.RedAmount = round2(-rec.ProjectedEnd)
	}
	if rec.HasGoal && rec.ProjectedEnd < cfg.MonthlyGoal {
		rec.BelowGoal = true
		rec.GoalGap = round2(cfg.MonthlyGoal - rec.ProjectedEnd)
	}
	return rec, nil
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
