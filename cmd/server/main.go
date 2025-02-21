package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	// logger configuration
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// configuration load
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// server init
	srvConfig := server.Config{
		Addr: cfg.Server.Addr,
	}
	srv := server.New(srvConfig, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Server
	if err := srv.Run(ctx); err != nil {
		logger.Error("Server run failed", "error", err)
		os.Exit(1)
	}
}
