package ggmax

import (
	"math"
	"testing"
)

const sample = `Tudo
Agendados
ID	Data	Descrição	Status	Valor	Saldo depois
#15344660	11/09/26 23:23
Pagamento #16109500, venda #J3EEL2E
Libera em 18/09/26 23:23
R$ 134,44
a liberar
—
#15214680	11/09/26 22:40
Pagamento #15971237, venda #3RM4OGQ
Concluído
R$ 100,82
saldo
saldo R$ 659,56
#15201417	11/09/26 01:25
Pagamento #15955590, venda #52ZGJB5
Concluído
R$ 252,06
saldo
saldo R$ 558,74
#15300164	09/09/26 13:14
Pagamento #16063220, venda #474Q30K
Libera em 16/09/26 13:14
R$ 100,82
a liberar
—`

func TestParseSample(t *testing.T) {
	txs := Parse(sample)
	if len(txs) != 4 {
		t.Fatalf("parsed %d transactions, want 4", len(txs))
	}

	first := txs[0]
	if first.ID != "15344660" {
		t.Errorf("id = %q, want 15344660", first.ID)
	}
	if first.VendaCode != "J3EEL2E" {
		t.Errorf("venda = %q, want J3EEL2E", first.VendaCode)
	}
	if first.Released {
		t.Errorf("first should be pending")
	}
	if math.Abs(first.Value-134.44) > 0.001 {
		t.Errorf("value = %.2f, want 134.44", first.Value)
	}
	if got := first.ReleaseDate.Format("2006-01-02"); got != "2026-09-18" {
		t.Errorf("release date = %s, want 2026-09-18", got)
	}

	// The second one is released; its value must be 100.82, not the 659.56 balance.
	if !txs[1].Released {
		t.Errorf("second should be released")
	}
	if math.Abs(txs[1].Value-100.82) > 0.001 {
		t.Errorf("released value = %.2f, want 100.82", txs[1].Value)
	}

	var released, pending float64
	for _, tx := range txs {
		if tx.Released {
			released += tx.Value
		} else {
			pending += tx.Value
		}
	}
	if math.Abs(released-352.88) > 0.001 { // 100.82 + 252.06
		t.Errorf("released total = %.2f, want 352.88", released)
	}
	if math.Abs(pending-235.26) > 0.001 { // 134.44 + 100.82
		t.Errorf("pending total = %.2f, want 235.26", pending)
	}
}
