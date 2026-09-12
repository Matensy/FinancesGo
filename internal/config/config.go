// Package config resolves runtime paths and server options.
package config

import (
	"os"
	"path/filepath"
)

// Config holds resolved runtime settings.
type Config struct {
	Addr       string // listen address, e.g. ":8080"
	DataDir    string // base directory for all persistent files
	DBPath     string // sqlite database file
	ExportsDir string // monthly exports
	BackupsDir string // automatic/manual backups
}

// Load builds a Config from flags/env with sensible defaults.
func Load(addr, dataDir string) (Config, error) {
	if dataDir == "" {
		dataDir = envOr("FINANCESGO_DATA", "data")
	}
	if addr == "" {
		addr = envOr("FINANCESGO_ADDR", ":8080")
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Addr:       addr,
		DataDir:    abs,
		DBPath:     filepath.Join(abs, "dados.db"),
		ExportsDir: filepath.Join(abs, "exports"),
		BackupsDir: filepath.Join(abs, "backups"),
	}
	for _, d := range []string{cfg.DataDir, cfg.ExportsDir, cfg.BackupsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
