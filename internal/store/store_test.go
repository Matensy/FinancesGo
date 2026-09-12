package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Matensy/FinancesGo/internal/database"
	"github.com/Matensy/FinancesGo/internal/finance"
	"github.com/Matensy/FinancesGo/internal/models"
	"github.com/Matensy/FinancesGo/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

func TestBalanceAndBillPayment(t *testing.T) {
	s := newStore(t)
	if err := s.UpdateFinanceSettings(1000, 0.1598, "R$", 7, 3); err != nil {
		t.Fatal(err)
	}
	eng := finance.New(s)

	if _, err := s.CreateIncome(models.Income{Description: "Freela", Amount: 500, Date: time.Now(), Category: "x", Confirmed: true, Source: "manual"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateExpense(models.Expense{Description: "Mercado", Amount: 150, Date: time.Now(), Category: "x", Source: "manual"}); err != nil {
		t.Fatal(err)
	}
	bal, err := eng.Balance()
	if err != nil {
		t.Fatal(err)
	}
	if bal != 1350 { // 1000 + 500 - 150
		t.Fatalf("balance = %.2f, want 1350", bal)
	}

	billID, err := s.CreateBill(models.Bill{Name: "Aluguel", Amount: 1200, DueDay: 10, Category: "Moradia", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	period := time.Now().Format("2006-01")
	if err := s.MarkBillPaid(billID, period, 1200); err != nil {
		t.Fatal(err)
	}
	bal, _ = eng.Balance()
	if bal != 150 { // 1350 - 1200
		t.Fatalf("balance after pay = %.2f, want 150", bal)
	}
	if err := s.MarkBillUnpaid(billID, period); err != nil {
		t.Fatal(err)
	}
	bal, _ = eng.Balance()
	if bal != 1350 {
		t.Fatalf("balance after unpay = %.2f, want 1350", bal)
	}
}

func TestSaleMaturationAndRefund(t *testing.T) {
	s := newStore(t)
	_ = s.UpdateFinanceSettings(0, 0.1598, "R$", 7, 3)
	eng := finance.New(s)

	id, err := s.CreatePokemon(models.PokemonAccount{Email: "a@b.com", BaseValue: 100, GGMaxRate: 0.1598})
	if err != nil {
		t.Fatal(err)
	}
	// Sold 10 days ago with a 7-day hold -> should be mature now.
	soldAt := time.Now().AddDate(0, 0, -10)
	if err := s.MarkSold(id, 115.98, soldAt, 7); err != nil {
		t.Fatal(err)
	}

	// Before maturation the value must NOT be in the balance.
	if bal, _ := eng.Balance(); bal != 0 {
		t.Fatalf("balance before maturation = %.2f, want 0", bal)
	}

	n, err := s.MatureSales(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("matured = %d, want 1", n)
	}
	if bal, _ := eng.Balance(); bal != 115.98 {
		t.Fatalf("balance after maturation = %.2f, want 115.98", bal)
	}
	// Running again must be idempotent.
	if n, _ := s.MatureSales(time.Now()); n != 0 {
		t.Fatalf("second maturation = %d, want 0", n)
	}

	// Refund after maturation must create a compensating expense (back to 0).
	if err := s.Refund(id, "comprador desistiu"); err != nil {
		t.Fatal(err)
	}
	if bal, _ := eng.Balance(); bal != 0 {
		t.Fatalf("balance after refund = %.2f, want 0", bal)
	}
	acc, _ := s.GetPokemon(id)
	if acc.Status != models.StatusReembolsada {
		t.Fatalf("status = %s, want reembolsada", acc.Status)
	}
}

func TestPendingSaleNotYetMature(t *testing.T) {
	s := newStore(t)
	_ = s.UpdateFinanceSettings(0, 0.1598, "R$", 7, 3)
	id, _ := s.CreatePokemon(models.PokemonAccount{BaseValue: 200, GGMaxRate: 0.1598})
	if err := s.MarkSold(id, 231.96, time.Now(), 7); err != nil {
		t.Fatal(err)
	}
	n, _ := s.MatureSales(time.Now())
	if n != 0 {
		t.Fatalf("matured = %d, want 0 (still within hold)", n)
	}
	// Pending value within the next 8 days should be reported.
	total, err := s.PendingSalesValue(time.Now(), time.Now().AddDate(0, 0, 8))
	if err != nil {
		t.Fatal(err)
	}
	if total != 231.96 {
		t.Fatalf("pending = %.2f, want 231.96", total)
	}
}

func TestRecurringIncomeMaterialization(t *testing.T) {
	s := newStore(t)
	_ = s.UpdateFinanceSettings(0, 0.1598, "R$", 7, 3)
	// A salary on day 1 is always due by "today".
	if _, err := s.CreateRecurringIncome(models.RecurringIncome{Name: "Salário", Amount: 3000, Day: 1, Category: "Salário", IsSalary: true, Active: true}); err != nil {
		t.Fatal(err)
	}
	n, err := s.MaterializeRecurringIncomes(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("materialized = %d, want 1", n)
	}
	// Idempotent within the same month.
	if n, _ := s.MaterializeRecurringIncomes(time.Now()); n != 0 {
		t.Fatalf("second materialization = %d, want 0", n)
	}
	if bal, _ := finance.New(s).Balance(); bal != 3000 {
		t.Fatalf("balance = %.2f, want 3000", bal)
	}
}

func TestProjectionDoesNotAccumulatePastBills(t *testing.T) {
	s := newStore(t)
	_ = s.UpdateFinanceSettings(1000, 0.1598, "R$", 7, 3)
	// A bill due on day 1 is always due by "today" in the current month.
	if _, err := s.CreateBill(models.Bill{Name: "Aluguel", Amount: 100, DueDay: 1, Category: "Moradia", Active: true}); err != nil {
		t.Fatal(err)
	}
	proj, err := finance.New(s).Projection()
	if err != nil {
		t.Fatal(err)
	}
	// "Hoje" must subtract only the current month's unpaid bill (100), never a
	// runaway accumulation of past months.
	if proj.Today.Available != 900 {
		t.Fatalf("today available = %.2f, want 900", proj.Today.Available)
	}
	if proj.Today.BillsDue != 100 {
		t.Fatalf("today bills due = %.2f, want 100", proj.Today.BillsDue)
	}
}

func TestGGMAXWalletSeparateFromBank(t *testing.T) {
	s := newStore(t)
	_ = s.UpdateFinanceSettings(100, 0.1598, "R$", 7, 3)
	eng := finance.New(s)
	now := time.Now()

	// A released GGMAX sale (dated in the past) and a pending one (future).
	released := "ggmax:1"
	pending := "ggmax:2"
	if _, err := s.CreateIncome(models.Income{Description: "Venda GGMAX #A", Amount: 200, Date: now.AddDate(0, 0, -2), Category: "Venda Pokémon GO", Confirmed: true, Source: "ggmax", ExternalID: released}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateIncome(models.Income{Description: "Venda GGMAX #B", Amount: 300, Date: now.AddDate(0, 0, 5), Category: "Venda Pokémon GO", Confirmed: true, Source: "ggmax", ExternalID: pending}); err != nil {
		t.Fatal(err)
	}

	// GGMAX money must NOT be in the bank balance.
	if bal, _ := eng.Balance(); bal != 100 {
		t.Fatalf("bank balance = %.2f, want 100 (GGMAX excluded)", bal)
	}
	w, err := s.GGMAXWalletState(now)
	if err != nil {
		t.Fatal(err)
	}
	if w.Available != 200 || w.Pending != 300 {
		t.Fatalf("wallet available=%.2f pending=%.2f, want 200/300", w.Available, w.Pending)
	}

	// Withdrawing 150 moves it to the bank and reduces GGMAX available.
	if _, err := s.RegisterGGMAXWithdrawal(150, now); err != nil {
		t.Fatal(err)
	}
	if bal, _ := eng.Balance(); bal != 250 {
		t.Fatalf("bank after withdrawal = %.2f, want 250", bal)
	}
	if w, _ := s.GGMAXWalletState(now); w.Available != 50 {
		t.Fatalf("available after withdrawal = %.2f, want 50", w.Available)
	}

	// Refunding (voiding) the released sale removes it from the wallet.
	sales, _ := s.ListGGMAXSales()
	var relID int64
	for _, sale := range sales {
		if sale.ExternalID == released {
			relID = sale.ID
		}
	}
	if err := s.SetIncomeVoided(relID, true); err != nil {
		t.Fatal(err)
	}
	if w, _ := s.GGMAXWalletState(now); w.Refunded != 200 {
		t.Fatalf("refunded = %.2f, want 200", w.Refunded)
	}
}

func TestGGMaxPriceCalculation(t *testing.T) {
	p := models.PokemonAccount{BaseValue: 100, GGMaxRate: 0.1598}
	if got := p.FinalPrice(); got != 115.98 {
		t.Fatalf("final price = %.2f, want 115.98", got)
	}
	if got := p.Fee(); got != 15.98 {
		t.Fatalf("fee = %.2f, want 15.98", got)
	}
}
