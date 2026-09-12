package server

import (
	"math"
	"testing"
)

func TestParseMoney(t *testing.T) {
	cases := map[string]float64{
		"2.995,51":     2995.51, // pt-BR
		"2,995.51":     2995.51, // US
		"1000":         1000,
		"1000,50":      1000.50,
		"1000.50":      1000.50,
		"R$ 1.234,56":  1234.56,
		"1.234.567,89": 1234567.89,
		"1,234,567.89": 1234567.89,
		"":             0,
		"-50,25":       -50.25,
		"15,98":        15.98,
	}
	for in, want := range cases {
		if got := parseMoney(in); math.Abs(got-want) > 0.001 {
			t.Errorf("parseMoney(%q) = %.2f, want %.2f", in, got, want)
		}
	}
}
