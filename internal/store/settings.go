package store

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Matensy/FinancesGo/internal/models"
)

const (
	keyInitialBalance = "initial_balance"
	keyGGMaxRate      = "ggmax_rate"
	keyCurrency       = "currency"
	keySaleHoldDays   = "sale_hold_days"
	keyBillAlertDays  = "bill_alert_days"
	keyPasswordHash   = "password_hash"
)

func (s *Store) getSetting(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting upserts a single setting value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Settings returns the full settings with sensible defaults.
func (s *Store) Settings() (models.Settings, error) {
	cfg := models.Settings{
		InitialBalance: 0,
		GGMaxRate:      0.1598,
		Currency:       "R$",
		SaleHoldDays:   7,
		BillAlertDays:  3,
	}
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return cfg, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return cfg, err
		}
		switch k {
		case keyInitialBalance:
			cfg.InitialBalance, _ = strconv.ParseFloat(v, 64)
		case keyGGMaxRate:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				cfg.GGMaxRate = f
			}
		case keyCurrency:
			if v != "" {
				cfg.Currency = v
			}
		case keySaleHoldDays:
			if n, err := strconv.Atoi(v); err == nil {
				cfg.SaleHoldDays = n
			}
		case keyBillAlertDays:
			if n, err := strconv.Atoi(v); err == nil {
				cfg.BillAlertDays = n
			}
		case keyPasswordHash:
			cfg.HasPassword = v != ""
		}
	}
	return cfg, rows.Err()
}

// PasswordHash returns the stored bcrypt hash, or empty if none set.
func (s *Store) PasswordHash() (string, error) {
	v, _, err := s.getSetting(keyPasswordHash)
	return v, err
}

// SetPasswordHash stores (or clears when empty) the local password hash.
func (s *Store) SetPasswordHash(hash string) error {
	return s.SetSetting(keyPasswordHash, hash)
}

// UpdateFinanceSettings persists the core finance settings.
func (s *Store) UpdateFinanceSettings(initialBalance, ggmaxRate float64, currency string, saleHoldDays, billAlertDays int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	set := func(k, v string) error {
		_, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v)
		return err
	}
	if err := set(keyInitialBalance, strconv.FormatFloat(initialBalance, 'f', 2, 64)); err != nil {
		return err
	}
	if err := set(keyGGMaxRate, strconv.FormatFloat(ggmaxRate, 'f', -1, 64)); err != nil {
		return err
	}
	if err := set(keyCurrency, currency); err != nil {
		return err
	}
	if err := set(keySaleHoldDays, strconv.Itoa(saleHoldDays)); err != nil {
		return err
	}
	if err := set(keyBillAlertDays, strconv.Itoa(billAlertDays)); err != nil {
		return err
	}
	return tx.Commit()
}
