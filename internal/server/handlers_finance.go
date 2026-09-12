package server

import (
	"net/http"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

func parseDate(s string) time.Time {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}
	return time.Now()
}

func categoryOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// --- Bills ---

func (a *App) handleBillCreate(w http.ResponseWriter, r *http.Request) {
	b := models.Bill{
		Name:     r.FormValue("name"),
		Amount:   parseMoney(r.FormValue("amount")),
		DueDay:   parseIntField(r.FormValue("due_day")),
		Category: categoryOr(r.FormValue("category"), "Outros"),
		Active:   true,
	}
	a.withCore(func(c *core) { _, _ = c.store.CreateBill(b) })
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleBillUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	b := models.Bill{
		ID:       id,
		Name:     r.FormValue("name"),
		Amount:   parseMoney(r.FormValue("amount")),
		DueDay:   parseIntField(r.FormValue("due_day")),
		Category: categoryOr(r.FormValue("category"), "Outros"),
		Active:   r.FormValue("active") == "on" || r.FormValue("active") == "1",
	}
	a.withCore(func(c *core) { _ = c.store.UpdateBill(b) })
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleBillDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.withCore(func(c *core) { _ = c.store.DeleteBill(id) })
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleBillPay(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	period := r.FormValue("period")
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	a.withCore(func(c *core) {
		bill, err := c.store.GetBill(id)
		if err != nil {
			return
		}
		amount := parseMoney(r.FormValue("amount"))
		if amount == 0 {
			amount = bill.Amount
		}
		_ = c.store.MarkBillPaid(id, period, amount)
	})
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleBillUnpay(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	period := r.FormValue("period")
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	a.withCore(func(c *core) { _ = c.store.MarkBillUnpaid(id, period) })
	redirectBack(w, r, "/financeiro")
}

// --- Recurring incomes ---

func (a *App) handleRecurringCreate(w http.ResponseWriter, r *http.Request) {
	rec := models.RecurringIncome{
		Name:     r.FormValue("name"),
		Amount:   parseMoney(r.FormValue("amount")),
		Day:      parseIntField(r.FormValue("day")),
		Category: categoryOr(r.FormValue("category"), "Salário"),
		IsSalary: r.FormValue("is_salary") == "on" || r.FormValue("is_salary") == "1",
		Active:   true,
	}
	a.withCore(func(c *core) { _, _ = c.store.CreateRecurringIncome(rec) })
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleRecurringDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.withCore(func(c *core) { _ = c.store.DeleteRecurringIncome(id) })
	redirectBack(w, r, "/financeiro")
}

// --- Incomes ---

func (a *App) handleIncomeCreate(w http.ResponseWriter, r *http.Request) {
	in := models.Income{
		Description: r.FormValue("description"),
		Amount:      parseMoney(r.FormValue("amount")),
		Date:        parseDate(r.FormValue("date")),
		Category:    categoryOr(r.FormValue("category"), "Outros"),
		Confirmed:   r.FormValue("confirmed") != "0",
		Source:      "manual",
	}
	a.withCore(func(c *core) { _, _ = c.store.CreateIncome(in) })
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleIncomeDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.withCore(func(c *core) { _ = c.store.DeleteIncome(id) })
	redirectBack(w, r, "/financeiro")
}

// --- Expenses ---

func (a *App) handleExpenseCreate(w http.ResponseWriter, r *http.Request) {
	e := models.Expense{
		Description: r.FormValue("description"),
		Amount:      parseMoney(r.FormValue("amount")),
		Date:        parseDate(r.FormValue("date")),
		Category:    categoryOr(r.FormValue("category"), "Outros"),
		Source:      "manual",
	}
	a.withCore(func(c *core) { _, _ = c.store.CreateExpense(e) })
	redirectBack(w, r, "/financeiro")
}

func (a *App) handleExpenseDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.withCore(func(c *core) { _ = c.store.DeleteExpense(id) })
	redirectBack(w, r, "/financeiro")
}

// --- GGMAX wallet ---

// handleGGMAXVoid toggles the refunded/void state of a GGMAX sale.
func (a *App) handleGGMAXVoid(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	void := r.FormValue("void") != "0"
	a.withCore(func(c *core) { _ = c.store.SetIncomeVoided(id, void) })
	redirectBack(w, r, "/financeiro")
}

// handleGGMAXWithdraw registers money moved from GGMAX to the bank.
func (a *App) handleGGMAXWithdraw(w http.ResponseWriter, r *http.Request) {
	amount := parseMoney(r.FormValue("amount"))
	date := parseDate(r.FormValue("date"))
	if amount > 0 {
		a.withCore(func(c *core) { _, _ = c.store.RegisterGGMAXWithdrawal(amount, date) })
	}
	redirectBack(w, r, "/financeiro")
}
