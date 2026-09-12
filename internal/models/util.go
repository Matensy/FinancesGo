package models

import (
	"math"
	"strings"
)

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}
