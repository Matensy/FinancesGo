package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Matensy/FinancesGo/internal/models"
)

func normalizeTags(s string) string {
	parts := strings.Split(s, ",")
	var clean []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, ", ")
}

func (a *App) formToPokemon(r *http.Request) models.PokemonAccount {
	rate := parseMoney(r.FormValue("ggmax_rate"))
	// The rate field is entered as a percentage (e.g. 15.98) -> 0.1598.
	if rate > 1 {
		rate = rate / 100
	}
	if rate <= 0 {
		a.withCore(func(c *core) {
			cfg, _ := c.store.Settings()
			rate = cfg.GGMaxRate
		})
	}
	return models.PokemonAccount{
		Email:        r.FormValue("email"),
		Level:        parseIntField(r.FormValue("level")),
		Team:         models.Team(r.FormValue("team")),
		Description:  r.FormValue("description"),
		Legendaries:  r.FormValue("legendaries"),
		Shinies:      parseIntField(r.FormValue("shinies")),
		PokemonCount: parseIntField(r.FormValue("pokemon_count")),
		BagCapacity:  parseIntField(r.FormValue("bag_capacity")),
		BaseValue:    parseMoney(r.FormValue("base_value")),
		GGMaxRate:    rate,
		Tags:         normalizeTags(r.FormValue("tags")),
	}
}

func (a *App) handlePokemonCreate(w http.ResponseWriter, r *http.Request) {
	p := a.formToPokemon(r)
	p.Status = models.StatusAtiva
	a.withCore(func(c *core) { _, _ = c.store.CreatePokemon(p) })
	redirectBack(w, r, "/pokemon")
}

func (a *App) handlePokemonUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	p := a.formToPokemon(r)
	p.ID = id
	a.withCore(func(c *core) { _ = c.store.UpdatePokemon(p) })
	redirectBack(w, r, fmt.Sprintf("/pokemon/%d", id))
}

func (a *App) handlePokemonDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.withCore(func(c *core) { _ = c.store.DeletePokemon(id) })
	http.Redirect(w, r, "/pokemon", http.StatusSeeOther)
}

func (a *App) handlePokemonProblem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	note := r.FormValue("note")
	a.withCore(func(c *core) { _ = c.store.ToggleProblem(id, note) })
	redirectBack(w, r, "/pokemon")
}

func (a *App) handlePokemonStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	status := models.PokemonStatus(r.FormValue("status"))
	a.withCore(func(c *core) { _ = c.store.SetStatus(id, status) })
	redirectBack(w, r, "/pokemon")
}

func (a *App) handlePokemonSell(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	soldAt := parseDate(r.FormValue("sold_at"))
	a.withCore(func(c *core) {
		acc, err := c.store.GetPokemon(id)
		if err != nil {
			return
		}
		value := parseMoney(r.FormValue("value"))
		if value == 0 {
			value = acc.FinalPrice()
		}
		cfg, _ := c.store.Settings()
		_ = c.store.MarkSold(id, value, soldAt, cfg.SaleHoldDays)
	})
	redirectBack(w, r, "/pokemon")
}

func (a *App) handlePokemonRefund(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	reason := r.FormValue("reason")
	a.withCore(func(c *core) { _ = c.store.Refund(id, reason) })
	redirectBack(w, r, "/pokemon")
}

// handlePokemonSimulate returns an HTMX fragment showing the balance impact of
// selling the given account now ("e se eu vender agora?").
func (a *App) handlePokemonSimulate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{}
	found := false
	a.withCore(func(c *core) {
		acc, err := c.store.GetPokemon(id)
		if err != nil {
			return
		}
		found = true
		current, projected, _ := c.finance.SimulateSale(acc.FinalPrice())
		cfg, _ := c.store.Settings()
		data["Account"] = acc
		data["Current"] = current
		data["Projected"] = projected
		data["Gain"] = acc.FinalPrice()
		data["Cfg"] = cfg
	})
	if !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = a.renderer.RenderPartial(w, "pokemon_detail.html", "simulation", data)
}
