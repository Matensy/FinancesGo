package store

import (
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
)

// lastDayOfMonth returns the number of days in the given year/month.
func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// dueDateFor returns the concrete due date for a bill's due_day in a period.
func dueDateFor(year int, month time.Month, dueDay int) time.Time {
	d := dueDay
	if last := lastDayOfMonth(year, month); d > last {
		d = last
	}
	if d < 1 {
		d = 1
	}
	return time.Date(year, month, d, 0, 0, 0, 0, time.Local)
}

// CreateBill inserts a new recurring bill.
func (s *Store) CreateBill(b models.Bill) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO bills (name, amount, due_day, category, active)
		VALUES (?, ?, ?, ?, ?)`, b.Name, b.Amount, b.DueDay, b.Category, boolToInt(b.Active))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateBill updates an existing bill definition.
func (s *Store) UpdateBill(b models.Bill) error {
	_, err := s.db.Exec(`UPDATE bills SET name=?, amount=?, due_day=?, category=?, active=? WHERE id=?`,
		b.Name, b.Amount, b.DueDay, b.Category, boolToInt(b.Active), b.ID)
	return err
}

// DeleteBill removes a bill and its payment history.
func (s *Store) DeleteBill(id int64) error {
	_, err := s.db.Exec(`DELETE FROM bills WHERE id = ?`, id)
	return err
}

// GetBill returns a single bill by id.
func (s *Store) GetBill(id int64) (models.Bill, error) {
	var b models.Bill
	var active int
	var created string
	err := s.db.QueryRow(`SELECT id, name, amount, due_day, category, active, created_at
		FROM bills WHERE id = ?`, id).
		Scan(&b.ID, &b.Name, &b.Amount, &b.DueDay, &b.Category, &active, &created)
	if err != nil {
		return b, err
	}
	b.Active = active == 1
	b.CreatedAt = parseTime(created)
	return b, nil
}

// ListBills returns all bill definitions ordered by due day.
func (s *Store) ListBills() ([]models.Bill, error) {
	rows, err := s.db.Query(`SELECT id, name, amount, due_day, category, active, created_at
		FROM bills ORDER BY due_day, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Bill
	for rows.Next() {
		var b models.Bill
		var active int
		var created string
		if err := rows.Scan(&b.ID, &b.Name, &b.Amount, &b.DueDay, &b.Category, &active, &created); err != nil {
			return nil, err
		}
		b.Active = active == 1
		b.CreatedAt = parseTime(created)
		out = append(out, b)
	}
	return out, rows.Err()
}

// paymentFor returns the payment (if any) for a bill in a period.
func (s *Store) paymentFor(billID int64, period string) (*models.BillPayment, error) {
	var p models.BillPayment
	var paidAt string
	err := s.db.QueryRow(`SELECT id, bill_id, period, amount, paid_at FROM bill_payments
		WHERE bill_id = ? AND period = ?`, billID, period).
		Scan(&p.ID, &p.BillID, &p.Period, &p.Amount, &paidAt)
	if err != nil {
		return nil, nil //nolint:nilerr // no payment is a valid state
	}
	p.PaidAt = parseTime(paidAt)
	return &p, nil
}

// MarkBillPaid records a payment for a bill in a period.
func (s *Store) MarkBillPaid(billID int64, period string, amount float64) error {
	_, err := s.db.Exec(`INSERT INTO bill_payments (bill_id, period, amount, paid_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(bill_id, period) DO UPDATE SET amount = excluded.amount, paid_at = excluded.paid_at`,
		billID, period, amount, fmtDateTime(time.Now()))
	return err
}

// MarkBillUnpaid removes a payment record for a bill in a period.
func (s *Store) MarkBillUnpaid(billID int64, period string) error {
	_, err := s.db.Exec(`DELETE FROM bill_payments WHERE bill_id = ? AND period = ?`, billID, period)
	return err
}

// BillsForPeriod returns bill views (with derived status) for the given period.
func (s *Store) BillsForPeriod(year int, month time.Month, category string) ([]models.BillView, error) {
	bills, err := s.ListBills()
	if err != nil {
		return nil, err
	}
	period := periodKey(year, month)
	today := startOfDay(time.Now())
	var out []models.BillView
	for _, b := range bills {
		if !b.Active {
			continue
		}
		if category != "" && b.Category != category {
			continue
		}
		bv := models.BillView{Bill: b, Period: period}
		bv.DueDate = dueDateFor(year, month, b.DueDay)
		pay, _ := s.paymentFor(b.ID, period)
		if pay != nil {
			bv.Paid = true
			bv.PaidAt = pay.PaidAt
			bv.Status = "paga"
		} else if bv.DueDate.Before(today) {
			bv.Status = "atrasada"
		} else {
			bv.Status = "a vencer"
		}
		bv.DaysUntil = int(bv.DueDate.Sub(today).Hours() / 24)
		out = append(out, bv)
	}
	return out, nil
}

// UnpaidBillsDueBy returns the total of unpaid bill amounts with a due date on
// or before "until" (inclusive), scanning from "from" forward month by month.
func (s *Store) UnpaidBillsDueBy(from, until time.Time) (float64, []models.BillView, error) {
	bills, err := s.ListBills()
	if err != nil {
		return 0, nil, err
	}
	from = startOfDay(from)
	until = startOfDay(until)
	var total float64
	var views []models.BillView
	// Iterate months from the earliest relevant month to the "until" month.
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.Local)
	end := time.Date(until.Year(), until.Month(), 1, 0, 0, 0, 0, time.Local)
	for !cur.After(end) {
		period := periodKey(cur.Year(), cur.Month())
		for _, b := range bills {
			if !b.Active {
				continue
			}
			due := dueDateFor(cur.Year(), cur.Month(), b.DueDay)
			if due.Before(from) || due.After(until) {
				continue
			}
			if pay, _ := s.paymentFor(b.ID, period); pay != nil {
				continue
			}
			total += b.Amount
			bv := models.BillView{Bill: b, Period: period, DueDate: due, Status: "a vencer"}
			if due.Before(startOfDay(time.Now())) {
				bv.Status = "atrasada"
			}
			bv.DaysUntil = int(due.Sub(startOfDay(time.Now())).Hours() / 24)
			views = append(views, bv)
		}
		cur = cur.AddDate(0, 1, 0)
	}
	return total, views, nil
}

func periodKey(year int, month time.Month) string {
	return time.Date(year, month, 1, 0, 0, 0, 0, time.Local).Format("2006-01")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
