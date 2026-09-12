package server

import (
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Matensy/FinancesGo/internal/models"
	"github.com/Matensy/FinancesGo/internal/web"
)

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	// Static assets.
	staticFS, _ := fs.Sub(web.Static, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Auth.
	mux.HandleFunc("GET /login", a.handleLoginPage)
	mux.HandleFunc("POST /login", a.handleLogin)
	mux.HandleFunc("POST /logout", a.handleLogout)

	// Pages.
	mux.HandleFunc("GET /{$}", a.requireAuth(a.handleDashboard))
	mux.HandleFunc("GET /financeiro", a.requireAuth(a.handleFinance))
	mux.HandleFunc("GET /pokemon", a.requireAuth(a.handlePokemonList))
	mux.HandleFunc("GET /pokemon/{id}", a.requireAuth(a.handlePokemonDetail))
	mux.HandleFunc("GET /relatorio", a.requireAuth(a.handleReport))
	mux.HandleFunc("GET /configuracoes", a.requireAuth(a.handleSettings))

	// Bill actions.
	mux.HandleFunc("POST /bills", a.requireAuth(a.handleBillCreate))
	mux.HandleFunc("POST /bills/{id}/update", a.requireAuth(a.handleBillUpdate))
	mux.HandleFunc("POST /bills/{id}/delete", a.requireAuth(a.handleBillDelete))
	mux.HandleFunc("POST /bills/{id}/pay", a.requireAuth(a.handleBillPay))
	mux.HandleFunc("POST /bills/{id}/unpay", a.requireAuth(a.handleBillUnpay))

	// Recurring incomes.
	mux.HandleFunc("POST /recurring-incomes", a.requireAuth(a.handleRecurringCreate))
	mux.HandleFunc("POST /recurring-incomes/{id}/delete", a.requireAuth(a.handleRecurringDelete))

	// Incomes.
	mux.HandleFunc("POST /incomes", a.requireAuth(a.handleIncomeCreate))
	mux.HandleFunc("POST /incomes/{id}/delete", a.requireAuth(a.handleIncomeDelete))

	// Expenses.
	mux.HandleFunc("POST /expenses", a.requireAuth(a.handleExpenseCreate))
	mux.HandleFunc("POST /expenses/{id}/delete", a.requireAuth(a.handleExpenseDelete))

	// Pokémon actions.
	mux.HandleFunc("POST /pokemon", a.requireAuth(a.handlePokemonCreate))
	mux.HandleFunc("POST /pokemon/{id}/update", a.requireAuth(a.handlePokemonUpdate))
	mux.HandleFunc("POST /pokemon/{id}/delete", a.requireAuth(a.handlePokemonDelete))
	mux.HandleFunc("POST /pokemon/{id}/problem", a.requireAuth(a.handlePokemonProblem))
	mux.HandleFunc("POST /pokemon/{id}/status", a.requireAuth(a.handlePokemonStatus))
	mux.HandleFunc("POST /pokemon/{id}/sell", a.requireAuth(a.handlePokemonSell))
	mux.HandleFunc("POST /pokemon/{id}/refund", a.requireAuth(a.handlePokemonRefund))
	mux.HandleFunc("GET /pokemon/{id}/simulate", a.requireAuth(a.handlePokemonSimulate))
	mux.HandleFunc("POST /import/ggmax", a.requireAuth(a.handleImportGGMAX))

	// Export.
	mux.HandleFunc("POST /export/xlsx", a.requireAuth(a.handleExportXLSX))
	mux.HandleFunc("POST /export/csv", a.requireAuth(a.handleExportCSV))
	mux.HandleFunc("GET /export/download", a.requireAuth(a.handleExportDownload))

	// Backup / settings.
	mux.HandleFunc("POST /backup/create", a.requireAuth(a.handleBackupCreate))
	mux.HandleFunc("GET /backup/export", a.requireAuth(a.handleBackupExport))
	mux.HandleFunc("POST /backup/import", a.requireAuth(a.handleBackupImport))
	mux.HandleFunc("POST /settings/finance", a.requireAuth(a.handleSettingsFinance))
	mux.HandleFunc("POST /settings/password", a.requireAuth(a.handleSettingsPassword))

	return logRequests(mux)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		_ = r
	})
}

// --- shared helpers ---

func (a *App) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.renderer.Render(w, page, data); err != nil {
		log.Printf("render %s: %v", page, err)
		http.Error(w, "Erro ao renderizar página: "+err.Error(), http.StatusInternalServerError)
	}
}

// baseData builds the common template data.
func (a *App) baseData(r *http.Request, title string) map[string]any {
	data := map[string]any{
		"Title":   title,
		"Section": sectionFor(r.URL.Path),
	}
	a.withCore(func(c *core) {
		cfg, _ := c.store.Settings()
		data["Cfg"] = cfg
	})
	if data["Cfg"] == nil {
		data["Cfg"] = models.Settings{Currency: "R$", GGMaxRate: 0.1598}
	}
	return data
}

func sectionFor(path string) string {
	switch {
	case path == "/" || path == "":
		return "dashboard"
	case strings.HasPrefix(path, "/financeiro"):
		return "financeiro"
	case strings.HasPrefix(path, "/pokemon"):
		return "pokemon"
	case strings.HasPrefix(path, "/relatorio"):
		return "relatorio"
	case strings.HasPrefix(path, "/configuracoes"):
		return "configuracoes"
	default:
		return ""
	}
}

func pathID(r *http.Request) (int64, bool) {
	return parseID(r.PathValue("id"))
}

func parseID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// parseMoney parses money in either "1.234,56" (pt-BR) or "1234.56" formats.
func parseMoney(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.TrimSpace(s)
	// If it has both '.' and ',', assume '.' is thousands and ',' decimal.
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseIntField(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func redirectBack(w http.ResponseWriter, r *http.Request, fallback string) {
	ref := r.FormValue("return")
	if ref == "" {
		ref = r.Referer()
	}
	if ref == "" {
		ref = fallback
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}
