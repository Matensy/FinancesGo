package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Matensy/FinancesGo/internal/backup"
	"golang.org/x/crypto/bcrypt"
)

// --- Export ---

func (a *App) handleExportXLSX(w http.ResponseWriter, r *http.Request) {
	var name string
	a.withCore(func(c *core) {
		res, err := c.exporter.ExportXLSX()
		if err == nil {
			name = res.Filename
		}
	})
	if name == "" {
		http.Redirect(w, r, "/configuracoes?erro=Falha+ao+exportar", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/export/download?name="+url.QueryEscape(name), http.StatusSeeOther)
}

func (a *App) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	var name string
	a.withCore(func(c *core) {
		res, err := c.exporter.ExportCSV()
		if err == nil {
			name = res.Filename
		}
	})
	if name == "" {
		http.Redirect(w, r, "/configuracoes?erro=Falha+ao+exportar", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/export/download?name="+url.QueryEscape(name), http.StatusSeeOther)
}

func (a *App) handleExportDownload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	var path string
	var ok bool
	a.withCore(func(c *core) { path, ok = c.exporter.FilePath(name) })
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(path)))
	http.ServeFile(w, r, path)
}

// --- Backup ---

func (a *App) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	var err error
	a.withCore(func(c *core) {
		_, err = c.backup.Create()
		if err == nil {
			_ = c.backup.Prune(20)
		}
	})
	if err != nil {
		http.Redirect(w, r, "/configuracoes?erro="+url.QueryEscape("Falha no backup: "+err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/configuracoes?saved=1&msg="+url.QueryEscape("Backup criado com sucesso"), http.StatusSeeOther)
}

func (a *App) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	var tmp string
	var err error
	a.withCore(func(c *core) { tmp, err = c.backup.SnapshotToTemp() })
	if err != nil {
		http.Error(w, "Falha ao gerar backup: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp)
	name := fmt.Sprintf("financesgo-backup-%s.db", time.Now().Format("2006-01-02-150405"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, tmp)
}

func (a *App) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Redirect(w, r, "/configuracoes?erro="+url.QueryEscape("Upload inválido"), http.StatusSeeOther)
		return
	}
	file, _, err := r.FormFile("dbfile")
	if err != nil {
		http.Redirect(w, r, "/configuracoes?erro="+url.QueryEscape("Selecione um arquivo .db"), http.StatusSeeOther)
		return
	}
	defer file.Close()

	// Snapshot current DB as a safety backup before replacing.
	a.withCore(func(c *core) { _, _ = c.backup.Create() })

	// Close current connection, replace the file, then reopen.
	a.mu.Lock()
	if a.core != nil {
		a.core.db.Close()
	}
	dbPath := a.cfg.DBPath
	a.mu.Unlock()

	if err := backup.Replace(file, dbPath); err != nil {
		// Reopen whatever is there so the app keeps working.
		_ = a.reload()
		http.Redirect(w, r, "/configuracoes?erro="+url.QueryEscape("Importação falhou: "+err.Error()), http.StatusSeeOther)
		return
	}
	if err := a.reload(); err != nil {
		http.Error(w, "Falha ao reabrir banco: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/configuracoes?saved=1&msg="+url.QueryEscape("Banco importado com sucesso"), http.StatusSeeOther)
}

// --- Settings ---

func (a *App) handleSettingsFinance(w http.ResponseWriter, r *http.Request) {
	initial := parseMoney(r.FormValue("initial_balance"))
	rate := parseMoney(r.FormValue("ggmax_rate"))
	if rate > 1 {
		rate = rate / 100
	}
	currency := r.FormValue("currency")
	if currency == "" {
		currency = "R$"
	}
	hold := parseIntField(r.FormValue("sale_hold_days"))
	if hold <= 0 {
		hold = 7
	}
	alert := parseIntField(r.FormValue("bill_alert_days"))
	if alert <= 0 {
		alert = 3
	}
	a.withCore(func(c *core) {
		_ = c.store.UpdateFinanceSettings(initial, rate, currency, hold, alert)
	})
	http.Redirect(w, r, "/configuracoes?saved=1&msg="+url.QueryEscape("Configurações salvas"), http.StatusSeeOther)
}

func (a *App) handleSettingsPassword(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	if action == "remove" {
		a.withCore(func(c *core) { _ = c.store.SetPasswordHash("") })
		http.Redirect(w, r, "/configuracoes?saved=1&msg="+url.QueryEscape("Senha removida"), http.StatusSeeOther)
		return
	}
	pwd := r.FormValue("password")
	confirm := r.FormValue("confirm")
	if pwd == "" || pwd != confirm {
		http.Redirect(w, r, "/configuracoes?erro="+url.QueryEscape("As senhas não conferem"), http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/configuracoes?erro="+url.QueryEscape("Falha ao definir senha"), http.StatusSeeOther)
		return
	}
	a.withCore(func(c *core) { _ = c.store.SetPasswordHash(string(hash)) })
	http.Redirect(w, r, "/configuracoes?saved=1&msg="+url.QueryEscape("Senha definida"), http.StatusSeeOther)
}
