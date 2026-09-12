// Command financesgo runs the FinancesGo web application: a personal finance
// dashboard integrated with a Pokémon GO account sales manager.
//
// It is a single binary that serves a local web UI (default http://localhost:8080)
// backed by a SQLite database file.
package main

import (
	"flag"
	"log"

	"github.com/Matensy/FinancesGo/internal/config"
	"github.com/Matensy/FinancesGo/internal/server"
)

func main() {
	addr := flag.String("addr", "", "endereço de escuta (ex: :8080)")
	dataDir := flag.String("data", "", "diretório de dados (padrão: ./data)")
	flag.Parse()

	cfg, err := config.Load(*addr, *dataDir)
	if err != nil {
		log.Fatalf("configuração: %v", err)
	}

	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("inicialização: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("servidor: %v", err)
	}
}
