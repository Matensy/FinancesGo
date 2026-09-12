package server

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"strings"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
	"github.com/Matensy/FinancesGo/internal/web"
)

// Renderer holds the parsed template sets (one per page, sharing the layout).
type Renderer struct {
	pages map[string]*template.Template
}

var funcMap = template.FuncMap{
	"money":     formatMoney,
	"num":       formatNumber,
	"pct":       func(f float64) string { return strings.ReplaceAll(fmt.Sprintf("%.2f%%", f*100), ".", ",") },
	"pctNum":    func(f float64) string { return strings.ReplaceAll(fmt.Sprintf("%.2f", f*100), ".", ",") },
	"date":      func(t time.Time) string { return safeDate(t, "02/01/2006") },
	"datetime":  func(t time.Time) string { return safeDate(t, "02/01/2006 15:04") },
	"dateISO":   func(t time.Time) string { return t.Format("2006-01-02") },
	"today":     func() string { return time.Now().Format("2006-01-02") },
	"teamLabel": teamLabel,
	"statusLbl": statusLabel,
	"statusCls": statusClass,
	"lower":     strings.ToLower,
	"title":     cases,
	"add":       func(a, b int) int { return a + b },
	"sub":       func(a, b float64) float64 { return a - b },
	"bagPct":    bagPercent,
	"hasPrefix": strings.HasPrefix,
	"dict":      dict,
	"nonEmpty":  func(s string) bool { return strings.TrimSpace(s) != "" },
	"deref":     derefFloat,
	"derefTime": derefTime,
	"initials":  initials,
	"icon":      iconSVG,
}

// iconSVG renders an inline SVG icon by name, referencing the sprite defined in
// the layout. Styling (size, color) is handled entirely by CSS.
func iconSVG(name string) template.HTML {
	return template.HTML(`<svg class="ic" aria-hidden="true" viewBox="0 0 24 24"><use href="#i-` + name + `"></use></svg>`)
}

// NewRenderer parses all templates from the embedded FS.
func NewRenderer() (*Renderer, error) {
	base, err := template.New("base").Funcs(funcMap).
		ParseFS(web.Templates, "templates/layout.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(web.Templates, "templates")
	if err != nil {
		return nil, err
	}
	r := &Renderer{pages: map[string]*template.Template{}}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "layout.html" {
			continue
		}
		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		t, err := clone.ParseFS(web.Templates, "templates/"+e.Name())
		if err != nil {
			return nil, err
		}
		r.pages[e.Name()] = t
	}
	return r, nil
}

// Render writes the named page wrapped in the layout.
func (r *Renderer) Render(w io.Writer, page string, data any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("template %q not found", page)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

// RenderPartial writes a standalone partial template (no layout).
func (r *Renderer) RenderPartial(w io.Writer, page, name string, data any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("template %q not found", page)
	}
	return t.ExecuteTemplate(w, name, data)
}

// --- formatting helpers ---

func formatNumber(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	intPart := int64(v)
	frac := int64(math.Round((v - float64(intPart)) * 100))
	if frac == 100 {
		intPart++
		frac = 0
	}
	s := fmt.Sprintf("%d", intPart)
	// group thousands with '.'
	var grouped strings.Builder
	n := len(s)
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(c)
	}
	res := fmt.Sprintf("%s,%02d", grouped.String(), frac)
	if neg {
		return "-" + res
	}
	return res
}

func formatMoney(v float64) string {
	return "R$ " + formatNumber(v)
}

func safeDate(t time.Time, layout string) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format(layout)
}

func teamLabel(t models.Team) string {
	switch t {
	case models.TeamInstinct:
		return "Instinct"
	case models.TeamMystic:
		return "Mystic"
	case models.TeamValor:
		return "Valor"
	default:
		return "—"
	}
}

func statusLabel(s models.PokemonStatus) string {
	switch s {
	case models.StatusAtiva:
		return "Ativa"
	case models.StatusAnunciada:
		return "Anunciada"
	case models.StatusVendida:
		return "Vendida"
	case models.StatusReembolsada:
		return "Reembolsada"
	case models.StatusProblema:
		return "Com problema"
	default:
		return string(s)
	}
}

func statusClass(s models.PokemonStatus) string {
	return "status-" + string(s)
}

func bagPercent(count, cap int) int {
	if cap <= 0 {
		return 0
	}
	p := int(math.Round(float64(count) / float64(cap) * 100))
	if p > 100 {
		return 100
	}
	return p
}

func cases(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict needs an even number of args")
	}
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		m[key] = values[i+1]
	}
	return m, nil
}

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func initials(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "?"
	}
	return strings.ToUpper(s[:1])
}
