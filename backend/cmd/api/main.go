package main

import (
	"log/slog"
	"os"

	"trainingops/internal/api"
	"trainingops/internal/config"
	"trainingops/internal/security"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	vault, err := security.NewVault(cfg.VaultMasterKey)
	if err != nil {
		logger.Error("vault init failed", "error", err)
		os.Exit(1)
	}

	server := api.NewServer(cfg, vault)

	if err := server.Start(cfg.HTTPAddr); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
