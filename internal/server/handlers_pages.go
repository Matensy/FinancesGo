package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
	"github.com/Matensy/FinancesGo/internal/store"
)

// selectedMonth parses the ?mes=YYYY-MM query param, defaulting to now.
func selectedMonth(r *http.Request) (int, time.Month, string) {
	now := time.Now()
	year, month := now.Year(), now.Month()
	if m := r.URL.Query().Get("mes"); m != "" {
		if t, err := time.Parse("2006-01", m); err == nil {
			year, month = t.Year(), t.Month()
		}
	}
	return year, month, time.Date(year, month, 1, 0, 0, 0, 0, time.Local).Format("2006-01")
}

func jsonStr(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := a.baseData(r, "Dashboard")
	year, month, _ := selectedMonth(r)

	a.withCore(func(c *core) {
		balance, _ := c.finance.Balance()
		proj, _ := c.finance.Projection()
		stats, _ := c.store.PokemonStats(year, month)

		incomes, _ := c.store.ListBankIncomesForMonth(year, month, "")
		expenses, _ := c.store.ListExpensesForMonth(year, month, "")
		paidBills, _ := c.store.PaidBillsForMonth(year, month)

		var incomeTotal, expenseTotal float64
		for _, in := range incomes {
			if in.Voided {
				continue
			}
			incomeTotal += in.Amount
		}
		for _, e := range expenses {
			expenseTotal += e.Amount
		}

		now := time.Now()
		_, upcoming, _ := c.store.UnpaidBillsDueBy(now, now.AddDate(0, 0, 30))
		cfg, _ := c.store.Settings()
		var alerts []models.BillView
		for _, b := range upcoming {
			if b.DaysUntil <= cfg.BillAlertDays {
				alerts = append(alerts, b)
			}
		}

		flow, _ := c.store.CashFlow(6)
		labels := make([]string, len(flow))
		inflow := make([]float64, len(flow))
		outflow := make([]float64, len(flow))
		for i, f := range flow {
			labels[i] = f.Label
			inflow[i] = f.Inflow
			outflow[i] = f.Outflow
		}

		wallet, _ := c.store.GGMAXWalletState(now)
		rec, _ := c.finance.Recommend()

		data["Balance"] = balance
		data["Projection"] = proj
		data["Stats"] = stats
		data["Wallet"] = wallet
		data["Rec"] = rec
		data["IncomeTotal"] = incomeTotal
		data["ExpenseTotal"] = expenseTotal
		data["PaidBills"] = paidBills
		data["MonthOut"] = expenseTotal + paidBills
		data["Upcoming"] = upcoming
		data["Alerts"] = alerts
		data["FlowLabels"] = jsonStr(labels)
		data["FlowInflow"] = jsonStr(inflow)
		data["FlowOutflow"] = jsonStr(outflow)
	})
	a.render(w, "dashboard.html", data)
}

func (a *App) handleFinance(w http.ResponseWriter, r *http.Request) {
	data := a.baseData(r, "Financeiro")
	year, month, period := selectedMonth(r)
	category := r.URL.Query().Get("categoria")

	a.withCore(func(c *core) {
		bills, _ := c.store.BillsForPeriod(year, month, category)
		billDefs, _ := c.store.ListBills()
		recurring, _ := c.store.ListRecurringIncomes()
		incomes, _ := c.store.ListBankIncomesForMonth(year, month, category)
		expenses, _ := c.store.ListExpensesForMonth(year, month, category)
		categories, _ := c.store.AllCategories()
		proj, _ := c.finance.Projection()
		balance, _ := c.finance.Balance()
		wallet, _ := c.store.GGMAXWalletState(time.Now())
		ggmaxSales, _ := c.store.ListGGMAXSales()

		var billsTotal, billsPaid, incomeTotal, expenseTotal float64
		for _, b := range bills {
			billsTotal += b.Amount
			if b.Paid {
				billsPaid += b.Amount
			}
		}
		for _, in := range incomes {
			incomeTotal += in.Amount
		}
		for _, e := range expenses {
			expenseTotal += e.Amount
		}

		data["Bills"] = bills
		data["BillDefs"] = billDefs
		data["Recurring"] = recurring
		data["Incomes"] = incomes
		data["Expenses"] = expenses
		data["Categories"] = categories
		data["Projection"] = proj
		data["Balance"] = balance
		data["BillsTotal"] = billsTotal
		data["BillsPaid"] = billsPaid
		data["IncomeTotal"] = incomeTotal
		data["ExpenseTotal"] = expenseTotal
		data["Period"] = period
		data["Category"] = category
		data["MonthOptions"] = monthOptions(6)
		data["Wallet"] = wallet
		data["GGMAXSales"] = ggmaxSales
	})
	data["Period"] = period
	a.render(w, "finance.html", data)
}

func (a *App) handlePokemonList(w http.ResponseWriter, r *http.Request) {
	data := a.baseData(r, "Pokémon GO")
	q := r.URL.Query()
	status := q.Get("status")
	team := q.Get("time")
	tag := q.Get("tag")
	year, month, _ := selectedMonth(r)

	a.withCore(func(c *core) {
		accounts, _ := c.store.ListPokemon(status, team, tag)
		stats, _ := c.store.PokemonStats(year, month)
		tags, _ := c.store.AllTags()
		cfg, _ := c.store.Settings()
		data["Accounts"] = accounts
		data["AccountsJSON"] = jsonStr(accounts)
		data["Stats"] = stats
		data["Tags"] = tags
		data["FilterStatus"] = status
		data["FilterTeam"] = team
		data["FilterTag"] = tag
		data["DefaultRate"] = cfg.GGMaxRate
	})
	data["ImportMsg"] = r.URL.Query().Get("import_msg")
	data["ImportErr"] = r.URL.Query().Get("import_err")
	a.render(w, "pokemon.html", data)
}

func (a *App) handlePokemonDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data := a.baseData(r, "Conta Pokémon GO")
	var found bool
	a.withCore(func(c *core) {
		acc, err := c.store.GetPokemon(id)
		if err != nil {
			return
		}
		found = true
		history, _ := c.store.PriceHistory(id)
		balance, _ := c.finance.Balance()
		data["Account"] = acc
		data["History"] = history
		data["Balance"] = balance
		data["ProjectedAfterSale"] = balance + acc.FinalPrice()
	})
	if !found {
		http.NotFound(w, r)
		return
	}
	a.render(w, "pokemon_detail.html", data)
}

func (a *App) handleReport(w http.ResponseWriter, r *http.Request) {
	data := a.baseData(r, "Relatório mensal")
	year, month, period := selectedMonth(r)

	a.withCore(func(c *core) {
		incomes, _ := c.store.ListIncomesForMonth(year, month, "")
		expenses, _ := c.store.ListExpensesForMonth(year, month, "")
		paidBills, _ := c.store.PaidBillsForMonth(year, month)
		byCat, _ := c.store.ExpenseByCategory(year, month)
		stats, _ := c.store.PokemonStats(year, month)
		exports, _ := c.store.ListExportRecords()

		var incomeTotal, expenseTotal, pokemonIncome float64
		for _, in := range incomes {
			if in.Voided {
				continue
			}
			// Raw GGMAX sales are platform money, not bank income; the bank
			// income is the withdrawal. Count earnings under pokemonIncome only.
			if in.Source != "ggmax" {
				incomeTotal += in.Amount
			}
			if in.Source == "pokemon" || in.Source == "ggmax" {
				pokemonIncome += in.Amount
			}
		}
		for _, e := range expenses {
			expenseTotal += e.Amount
		}
		totalOut := expenseTotal + paidBills

		// Sold accounts this month for ticket médio.
		soldCount := 0
		var soldSum float64
		all, _ := c.store.ListPokemon("", "", "")
		start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, 0)
		var problems []models.PokemonAccount
		for _, p := range all {
			if p.SoldAt != nil && !p.SoldAt.Before(start) && p.SoldAt.Before(end) && p.SoldValue != nil {
				soldCount++
				soldSum += *p.SoldValue
			}
			if p.Status == models.StatusProblema {
				problems = append(problems, p)
			}
		}
		ticket := 0.0
		if soldCount > 0 {
			ticket = soldSum / float64(soldCount)
		}

		catLabels := make([]string, len(byCat))
		catValues := make([]float64, len(byCat))
		for i, ct := range byCat {
			catLabels[i] = ct.Category
			catValues[i] = ct.Amount
		}

		data["Incomes"] = incomes
		data["Expenses"] = expenses
		data["IncomeTotal"] = incomeTotal
		data["ExpenseTotal"] = expenseTotal
		data["PaidBills"] = paidBills
		data["TotalOut"] = totalOut
		data["Net"] = incomeTotal - totalOut
		data["PokemonIncome"] = pokemonIncome
		data["ByCategory"] = byCat
		data["Stats"] = stats
		data["Exports"] = exports
		data["SoldCount"] = soldCount
		data["SoldSum"] = soldSum
		data["Ticket"] = ticket
		data["Problems"] = problems
		data["CatLabels"] = jsonStr(catLabels)
		data["CatValues"] = jsonStr(catValues)
		data["Period"] = period
		data["MonthOptions"] = monthOptions(12)
	})
	data["Period"] = period
	a.render(w, "report.html", data)
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	data := a.baseData(r, "Configurações")
	data["Saved"] = r.URL.Query().Get("saved") != ""
	data["Msg"] = r.URL.Query().Get("msg")
	data["Err"] = r.URL.Query().Get("erro")
	a.withCore(func(c *core) {
		exports, _ := c.store.ListExportRecords()
		data["Exports"] = exports
		data["Backups"], _ = c.backup.List()
	})
	data["DataDir"] = a.cfg.DataDir
	data["DBPath"] = a.cfg.DBPath
	a.render(w, "settings.html", data)
}

// monthOptions returns the last n months as {Value:"2006-01", Label:"set/26"} for selects.
func monthOptions(n int) []map[string]string {
	now := time.Now()
	out := make([]map[string]string, 0, n)
	for i := 0; i < n; i++ {
		m := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -i, 0)
		out = append(out, map[string]string{
			"Value": m.Format("2006-01"),
			"Label": store.MonthLabel(m),
		})
	}
	return out
}
