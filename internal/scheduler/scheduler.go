// Package scheduler runs the periodic maintenance jobs: maturing Pokémon sales
// into the balance after the hold period and materializing recurring incomes.
package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/Matensy/FinancesGo/internal/store"
)

// Scheduler periodically runs idempotent maintenance jobs.
type Scheduler struct {
	store    *store.Store
	interval time.Duration
}

// New returns a Scheduler. The jobs are idempotent, so a short interval is safe.
func New(s *store.Store) *Scheduler {
	return &Scheduler{store: s, interval: time.Hour}
}

// RunOnce executes the maintenance jobs a single time.
func (sc *Scheduler) RunOnce() {
	now := time.Now()
	if n, err := sc.store.MatureSales(now); err != nil {
		log.Printf("scheduler: mature sales: %v", err)
	} else if n > 0 {
		log.Printf("scheduler: %d Pokémon sale(s) creditadas ao saldo", n)
	}
	if n, err := sc.store.MaterializeRecurringIncomes(now); err != nil {
		log.Printf("scheduler: materialize recurring incomes: %v", err)
	} else if n > 0 {
		log.Printf("scheduler: %d receita(s) recorrente(s) lançada(s)", n)
	}
}

// Start runs the maintenance loop until ctx is cancelled. It runs once
// immediately and then on every tick.
func (sc *Scheduler) Start(ctx context.Context) {
	sc.RunOnce()
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.RunOnce()
		}
	}
}
