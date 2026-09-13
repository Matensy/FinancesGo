// Package models defines the domain types shared across the application.
package models

import "time"

// Team represents a Pokémon GO team.
type Team string

const (
	TeamInstinct Team = "instinct"
	TeamMystic   Team = "mystic"
	TeamValor    Team = "valor"
	TeamNone     Team = ""
)

// PokemonStatus represents the lifecycle state of a Pokémon GO account.
type PokemonStatus string

const (
	StatusAtiva       PokemonStatus = "ativa"
	StatusAnunciada   PokemonStatus = "anunciada"
	StatusVendida     PokemonStatus = "vendida"
	StatusReembolsada PokemonStatus = "reembolsada"
	StatusProblema    PokemonStatus = "problema"
)

// Settings holds the application configuration stored in the settings table.
type Settings struct {
	InitialBalance float64
	GGMaxRate      float64 // e.g. 0.1598 for 15,98%
	Currency       string
	SaleHoldDays   int  // days before a sale matures into the balance (default 7)
	BillAlertDays  int  // how many days ahead to alert about upcoming bills
	HasPassword    bool // whether a local password is configured

	CardLimit   float64 // credit card total limit
	CardUsed    float64 // amount currently used on the card (fatura)
	CardDueDay  int     // day of month the card bill is due (default 5)
	MonthlyGoal float64 // desired balance at the end of each month
	DailyGoal   float64 // target earnings per day (GGMAX sales)
}

// CardAvailable returns the remaining credit card limit.
func (s Settings) CardAvailable() float64 {
	v := s.CardLimit - s.CardUsed
	if v < 0 {
		return 0
	}
	return v
}

// CardUsedPct returns how much of the limit is used (0..100).
func (s Settings) CardUsedPct() int {
	if s.CardLimit <= 0 {
		return 0
	}
	p := int(s.CardUsed / s.CardLimit * 100)
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}

// Bill is a recurring fixed monthly expense definition.
type Bill struct {
	ID        int64
	Name      string
	Amount    float64
	DueDay    int // 1..31
	Category  string
	Active    bool
	CreatedAt time.Time
}

// BillPayment records that a bill was paid for a given period (YYYY-MM).
type BillPayment struct {
	ID        int64
	BillID    int64
	Period    string // "2026-09"
	Amount    float64
	PaidAt    time.Time
	CreatedAt time.Time
}

// BillView is a bill projected onto a specific period, with derived status.
type BillView struct {
	Bill
	Period    string
	DueDate   time.Time
	Paid      bool
	PaidAt    time.Time
	Status    string // "paga" | "a vencer" | "atrasada"
	DaysUntil int    // negative if overdue
}

// RecurringIncome is a recurring income template (e.g. salary).
type RecurringIncome struct {
	ID        int64
	Name      string
	Amount    float64
	Day       int // day of month
	Category  string
	IsSalary  bool
	Active    bool
	CreatedAt time.Time
}

// Income is a concrete income entry (materialized or one-off).
type Income struct {
	ID          int64
	Description string
	Amount      float64
	Date        time.Time
	Category    string
	Confirmed   bool
	Source      string // "manual" | "recurring" | "pokemon" | "ggmax" | "ggmax_withdraw"
	PokemonID   *int64
	Period      *string    // set for materialized recurring incomes
	ExternalID  string     // dedup key for imported entries (e.g. GGMAX tx id)
	Voided      bool       // refunded/cancelled: counts nowhere
	SaleDate    *time.Time // for GGMAX sales: the day sold (distinct from release date)
	CreatedAt   time.Time
}

// Expense is a one-off expense (avulso) or an automatic Pokémon refund.
type Expense struct {
	ID          int64
	Description string
	Amount      float64
	Date        time.Time
	Category    string
	Source      string // "manual" | "pokemon"
	PokemonID   *int64
	CreatedAt   time.Time
}

// PokemonAccount is a Pokémon GO account tracked for sale.
type PokemonAccount struct {
	ID           int64
	Email        string
	Level        int
	Team         Team
	Description  string
	Legendaries  string // free text / list
	Shinies      int
	PokemonCount int
	BagCapacity  int
	BaseValue    float64
	GGMaxRate    float64
	Tags         string // comma separated
	Status       PokemonStatus
	ProblemNote  string
	ProblemAt    *time.Time
	SoldAt       *time.Time
	SoldValue    *float64
	MaturesAt    *time.Time
	Matured      bool
	RefundedAt   *time.Time
	RefundReason string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FinalPrice returns the sale price including the GGMAX fee.
func (p PokemonAccount) FinalPrice() float64 {
	return round2(p.BaseValue * (1 + p.GGMaxRate))
}

// Fee returns the GGMAX fee amount.
func (p PokemonAccount) Fee() float64 {
	return round2(p.BaseValue * p.GGMaxRate)
}

// TagList splits the comma-separated tags into a slice.
func (p PokemonAccount) TagList() []string {
	var out []string
	for _, t := range splitComma(p.Tags) {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Earning is a manually logged daily earning that drives the goal tracker.
type Earning struct {
	ID        int64
	Date      time.Time
	Amount    float64
	Note      string
	CreatedAt time.Time
}

// PriceHistory records a base value / final price snapshot over time.
type PriceHistory struct {
	ID         int64
	AccountID  int64
	BaseValue  float64
	GGMaxRate  float64
	FinalPrice float64
	RecordedAt time.Time
}

// ExportRecord is an entry in the export history.
type ExportRecord struct {
	ID           int64
	Filename     string
	Format       string
	Path         string
	Period       string
	AccountCount int
	CreatedAt    time.Time
}

// ProjectionPoint is one point on the "how much can I spend" timeline.
type ProjectionPoint struct {
	Label         string
	Date          time.Time
	Available     float64
	IncomingTotal float64
	BillsDue      float64
}

// Projection is the full spending projection.
type Projection struct {
	Balance   float64
	Points    []ProjectionPoint
	Today     ProjectionPoint
	Generated time.Time
}
