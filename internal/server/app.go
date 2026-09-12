// Package server wires the HTTP layer, sessions, and application core together.
package server

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"sync"

	"github.com/Matensy/FinancesGo/internal/backup"
	"github.com/Matensy/FinancesGo/internal/config"
	"github.com/Matensy/FinancesGo/internal/database"
	"github.com/Matensy/FinancesGo/internal/export"
	"github.com/Matensy/FinancesGo/internal/finance"
	"github.com/Matensy/FinancesGo/internal/scheduler"
	"github.com/Matensy/FinancesGo/internal/store"
)

// core groups the components that depend on the live database connection so
// they can be swapped atomically when the user imports a new database file.
type core struct {
	db       *sql.DB
	store    *store.Store
	finance  *finance.Engine
	exporter *export.Exporter
	backup   *backup.Manager
}

// App is the running application.
type App struct {
	cfg      config.Config
	renderer *Renderer
	sessions *sessionStore

	mu   sync.RWMutex
	core *core

	schedCancel context.CancelFunc
}

// New constructs an App: opens the DB, builds the core, parses templates.
func New(cfg config.Config) (*App, error) {
	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	app := &App{
		cfg:      cfg,
		renderer: renderer,
		sessions: newSessionStore(),
	}
	c, err := app.buildCore()
	if err != nil {
		return nil, err
	}
	app.core = c
	return app, nil
}

func (a *App) buildCore() (*core, error) {
	db, err := database.Open(a.cfg.DBPath)
	if err != nil {
		return nil, err
	}
	st := store.New(db)
	exp, err := export.New(st, a.cfg.ExportsDir)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &core{
		db:       db,
		store:    st,
		finance:  finance.New(st),
		exporter: exp,
		backup:   backup.New(db, a.cfg.DBPath, a.cfg.BackupsDir),
	}, nil
}

// withCore runs fn with a read-locked snapshot of the core.
func (a *App) withCore(fn func(c *core)) {
	a.mu.RLock()
	c := a.core
	a.mu.RUnlock()
	fn(c)
}

// reload closes the current core and rebuilds it (used after DB import).
func (a *App) reload() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.core
	if old != nil {
		old.db.Close()
	}
	c, err := a.buildCore()
	if err != nil {
		return err
	}
	a.core = c
	// Restart the scheduler against the new core.
	if a.schedCancel != nil {
		a.schedCancel()
	}
	a.startScheduler()
	return nil
}

func (a *App) startScheduler() {
	ctx, cancel := context.WithCancel(context.Background())
	a.schedCancel = cancel
	a.mu.RLock()
	st := a.core.store
	a.mu.RUnlock()
	go scheduler.New(st).Start(ctx)
}

// Run starts the scheduler and the HTTP server (blocking).
func (a *App) Run() error {
	a.startScheduler()
	srv := &http.Server{
		Addr:    a.cfg.Addr,
		Handler: a.routes(),
	}
	log.Printf("FinancesGo rodando em http://localhost%s", a.cfg.Addr)
	log.Printf("Banco de dados: %s", a.cfg.DBPath)
	return srv.ListenAndServe()
}
