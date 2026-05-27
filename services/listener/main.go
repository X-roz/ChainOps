package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"listener/config"
	"listener/providers"
	"listener/service"
)

func main() {

	// Load configuration: default path is "application.yml", but can be overridden with the -config flag
	configPath := flag.String("config", "application.yml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return
	}

	slog.Info("config loaded", "network", cfg.Network)

	// pass the RPC URLs to the providers package to establish connection
	providerList, err := providers.Connect(&cfg.RPCURLs)
	if err != nil {
		slog.Error("failed to connect to providers", "error", err)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service.ListenToBlocks(ctx, &providerList, cfg.SafeBlockBuffer)

}
