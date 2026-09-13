// Package ggmax parses the transaction list copied from the GGMAX website into
// structured sales that can be imported into the finance ledger.
package ggmax

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Tx is a single parsed GGMAX transaction.
type Tx struct {
	ID          string    // GGMAX transaction id, e.g. "15344660"
	VendaCode   string    // sale code, e.g. "J3EEL2E"
	Date        time.Time // transaction date/time
	Released    bool      // true = "Concluído" (already in balance)
	ReleaseDate time.Time // when a pending sale becomes available ("Libera em")
	Value       float64
}

// AvailableDate returns the date the money is (or becomes) available: the
// transaction date for released sales, the release date for pending ones.
func (t Tx) AvailableDate() time.Time {
	if t.Released {
		return t.Date
	}
	return t.ReleaseDate
}

const dtLayout = "02/01/06 15:04"

var (
	// A record starts with "#<digits>" followed by a "dd/mm/yy hh:mm" date.
	reStart   = regexp.MustCompile(`#\d+\s+\d{2}/\d{2}/\d{2}\s+\d{2}:\d{2}`)
	reID      = regexp.MustCompile(`#(\d+)`)
	reDate    = regexp.MustCompile(`(\d{2}/\d{2}/\d{2}\s+\d{2}:\d{2})`)
	reVenda   = regexp.MustCompile(`venda\s+#([A-Za-z0-9]+)`)
	reRelease = regexp.MustCompile(`Libera em\s+(\d{2}/\d{2}/\d{2}\s+\d{2}:\d{2})`)
	reValue   = regexp.MustCompile(`R\$\s*([0-9.]*[0-9],[0-9]{2})`)
)

// Parse extracts all transactions from raw pasted GGMAX text. Malformed records
// are skipped. Order follows the input.
func Parse(raw string) []Tx {
	starts := reStart.FindAllStringIndex(raw, -1)
	var out []Tx
	for i, s := range starts {
		end := len(raw)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		if tx, ok := parseBlock(raw[s[0]:end]); ok {
			out = append(out, tx)
		}
	}
	return out
}

func parseBlock(block string) (Tx, bool) {
	var tx Tx
	if m := reID.FindStringSubmatch(block); m != nil {
		tx.ID = m[1]
	} else {
		return tx, false
	}
	dates := reDate.FindAllStringSubmatch(block, -1)
	if len(dates) == 0 {
		return tx, false
	}
	tx.Date = parseDT(dates[0][1])

	if m := reVenda.FindStringSubmatch(block); m != nil {
		tx.VendaCode = strings.ToUpper(m[1])
	}
	if m := reRelease.FindStringSubmatch(block); m != nil {
		tx.Released = false
		tx.ReleaseDate = parseDT(m[1])
	} else if strings.Contains(block, "Conclu") {
		tx.Released = true
	} else {
		// Unknown status: skip rather than guess.
		return tx, false
	}

	if m := reValue.FindStringSubmatch(block); m != nil {
		tx.Value = parseBRL(m[1])
	}
	if tx.Value <= 0 || tx.Date.IsZero() {
		return tx, false
	}
	return tx, true
}

func parseDT(s string) time.Time {
	s = strings.Join(strings.Fields(s), " ")
	t, err := time.ParseInLocation(dtLayout, s, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseBRL(s string) float64 {
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseMoneyLoose parses a value in pt-BR or US format (decimal separator is
// whichever of '.' or ',' comes last).
func parseMoneyLoose(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, "R$", ""))
	lastDot := strings.LastIndex(s, ".")
	lastComma := strings.LastIndex(s, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastComma > lastDot {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case lastComma >= 0:
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// Simple is a lightweight "order + value" GGMAX entry.
type Simple struct {
	Code  string
	Value float64
}

var (
	reSimpleVal  = regexp.MustCompile(`(\d[\d.,]*[.,]\d{2})`)
	reSimpleNum  = regexp.MustCompile(`\d+`)
	reSimpleCode = regexp.MustCompile(`#?([A-Za-z0-9]{3,})`)
)

// ParseSimple parses lines of "<order> <value>" (in any order, with optional #,
// "R$", separators like "-" or ":"). One entry per line. It is used when the
// user types just the order number and the account's value instead of pasting
// the whole GGMAX transaction block.
func ParseSimple(raw string) []Simple {
	var out []Simple
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		valStr := ""
		if m := reSimpleVal.FindAllString(line, -1); len(m) > 0 {
			valStr = m[len(m)-1]
		} else if m := reSimpleNum.FindAllString(line, -1); len(m) > 0 {
			valStr = m[len(m)-1]
		}
		if valStr == "" {
			continue
		}
		value := parseMoneyLoose(valStr)
		rest := strings.Replace(line, valStr, " ", 1)
		code := ""
		if m := reSimpleCode.FindStringSubmatch(rest); m != nil {
			code = strings.ToUpper(m[1])
		}
		if code == "" || value <= 0 {
			continue
		}
		out = append(out, Simple{Code: code, Value: value})
	}
	return out
}
