// Package export produces .xlsx and .csv exports of the Pokémon accounts and
// keeps a history of exports on disk.
package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Matensy/FinancesGo/internal/models"
	"github.com/Matensy/FinancesGo/internal/store"
	"github.com/xuri/excelize/v2"
)

// Exporter writes export files into a directory and records them in the store.
type Exporter struct {
	store *store.Store
	dir   string
}

// New creates an Exporter that writes into dir (created if missing).
func New(s *store.Store, dir string) (*Exporter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Exporter{store: s, dir: dir}, nil
}

var headers = []string{
	"ID", "Email", "Level", "Time", "Descrição", "Lendários", "Shinys",
	"Qtd Pokémons", "Capacidade Bag", "Valor Base", "Taxa GGMAX", "Preço Final",
	"Status", "Observação Problema", "Data Venda", "Valor Venda", "Data Reembolso", "Tags",
}

func rowFor(p models.PokemonAccount) []string {
	soldAt, soldVal, refundAt := "", "", ""
	if p.SoldAt != nil {
		soldAt = p.SoldAt.Format("2006-01-02")
	}
	if p.SoldValue != nil {
		soldVal = strconv.FormatFloat(*p.SoldValue, 'f', 2, 64)
	}
	if p.RefundedAt != nil {
		refundAt = p.RefundedAt.Format("2006-01-02")
	}
	return []string{
		strconv.FormatInt(p.ID, 10),
		p.Email,
		strconv.Itoa(p.Level),
		string(p.Team),
		p.Description,
		p.Legendaries,
		strconv.Itoa(p.Shinies),
		strconv.Itoa(p.PokemonCount),
		strconv.Itoa(p.BagCapacity),
		strconv.FormatFloat(p.BaseValue, 'f', 2, 64),
		strconv.FormatFloat(p.GGMaxRate*100, 'f', 2, 64) + "%",
		strconv.FormatFloat(p.FinalPrice(), 'f', 2, 64),
		string(p.Status),
		p.ProblemNote,
		soldAt,
		soldVal,
		refundAt,
		p.Tags,
	}
}

// Result describes a produced export file.
type Result struct {
	Filename string
	Path     string
	Format   string
	Count    int
}

func (e *Exporter) accounts() ([]models.PokemonAccount, error) {
	return e.store.ListPokemon("", "", "")
}

// ExportXLSX writes all accounts to an .xlsx file and records it.
func (e *Exporter) ExportXLSX() (Result, error) {
	accs, err := e.accounts()
	if err != nil {
		return Result{}, err
	}
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Contas Pokémon GO"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return Result{}, err
	}
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")

	headStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"8B5CF6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headStyle)
	}
	for r, p := range accs {
		for c, val := range rowFor(p) {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	f.SetColWidth(sheet, "A", "R", 16)
	f.SetColWidth(sheet, "E", "F", 30)

	name := fmt.Sprintf("contas-pokemon-%s.xlsx", time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(e.dir, name)
	if err := f.SaveAs(path); err != nil {
		return Result{}, err
	}
	e.record(name, path, "xlsx", len(accs))
	return Result{Filename: name, Path: path, Format: "xlsx", Count: len(accs)}, nil
}

// ExportCSV writes all accounts to a .csv file and records it.
func (e *Exporter) ExportCSV() (Result, error) {
	accs, err := e.accounts()
	if err != nil {
		return Result{}, err
	}
	name := fmt.Sprintf("contas-pokemon-%s.csv", time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(e.dir, name)
	file, err := os.Create(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()
	file.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel compatibility
	w := csv.NewWriter(file)
	w.Comma = ';'
	if err := w.Write(headers); err != nil {
		return Result{}, err
	}
	for _, p := range accs {
		if err := w.Write(rowFor(p)); err != nil {
			return Result{}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return Result{}, err
	}
	e.record(name, path, "csv", len(accs))
	return Result{Filename: name, Path: path, Format: "csv", Count: len(accs)}, nil
}

func (e *Exporter) record(name, path, format string, count int) {
	_, _ = e.store.AddExportRecord(models.ExportRecord{
		Filename:     name,
		Format:       format,
		Path:         path,
		Period:       time.Now().Format("2006-01"),
		AccountCount: count,
	})
}

// FilePath returns the absolute path of an export file by name if it exists in
// the export directory (guards against path traversal).
func (e *Exporter) FilePath(name string) (string, bool) {
	clean := filepath.Base(name)
	path := filepath.Join(e.dir, clean)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}
