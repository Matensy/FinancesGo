package store

import (
	"fmt"
	"time"

	"github.com/Matensy/FinancesGo/internal/ggmax"
	"github.com/Matensy/FinancesGo/internal/models"
)

// GGMAXImportResult summarizes a GGMAX import run.
type GGMAXImportResult struct {
	Imported      int
	Skipped       int
	ReleasedValue float64 // total already available (dated in the past)
	PendingValue  float64 // total that will release in the future
}

// AddGGMAXOrders records GGMAX sales from simple "order + value" entries. When
// released is false the sale is pending and matures (goes to the balance) after
// holdDays; when true it is already available. Dedup is by order number.
func (s *Store) AddGGMAXOrders(items []ggmax.Simple, saleDate time.Time, released bool, holdDays int) (GGMAXImportResult, error) {
	txs := make([]ggmax.Tx, 0, len(items))
	for _, it := range items {
		tx := ggmax.Tx{ID: it.Code, VendaCode: it.Code, Date: saleDate, Value: it.Value, Released: released}
		if !released {
			tx.ReleaseDate = startOfDay(saleDate).AddDate(0, 0, holdDays)
		}
		txs = append(txs, tx)
	}
	return s.ImportGGMAX(txs)
}

// ImportGGMAX inserts each transaction as a "Venda Pokémon GO" income, keyed by
// the GGMAX transaction id so re-importing the same list is idempotent.
//
// Released ("Concluído") sales are dated at the transaction date and count in
// the balance immediately. Pending ("Libera em") sales are dated at their
// release date, so they enter the balance automatically on that day.
func (s *Store) ImportGGMAX(txs []ggmax.Tx) (GGMAXImportResult, error) {
	var res GGMAXImportResult
	for _, tx := range txs {
		extID := "ggmax:" + tx.ID
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM incomes WHERE external_id = ?`, extID).Scan(&exists); err != nil {
			return res, err
		}
		if exists > 0 {
			res.Skipped++
			continue
		}
		desc := "Venda GGMAX"
		if tx.VendaCode != "" {
			desc = "Venda GGMAX #" + tx.VendaCode
		}
		if _, err := s.CreateIncome(models.Income{
			Description: desc,
			Amount:      tx.Value,
			Date:        tx.AvailableDate(),
			Category:    "Venda Pokémon GO",
			Confirmed:   true,
			Source:      "ggmax",
			ExternalID:  extID,
		}); err != nil {
			return res, fmt.Errorf("importar %s: %w", tx.ID, err)
		}
		res.Imported++
		if tx.Released {
			res.ReleasedValue += tx.Value
		} else {
			res.PendingValue += tx.Value
		}
	}
	return res, nil
}
