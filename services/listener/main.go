package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"listener/config"
	"listener/db"
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
		os.Exit(1)
	}

	slog.Info("config loaded", "network", cfg.Network)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Connect(ctx, cfg.Database); err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if _, err := db.GetNetworkIDByKey(ctx, cfg.Network); err != nil {
		slog.Error("network not found or inactive in database, stopping listener", "network", cfg.Network)
		os.Exit(1)
	}
	slog.Info("network validated", "network", cfg.Network)

	if cfg.EvmBlockListen {
		if len(cfg.RPCURLs) == 0 {
			slog.Error("evm-block-listen is enabled but no rpc-urls configured")
			os.Exit(1)
		}
		providerList, err := providers.ConnectEVM(ctx, cfg.RPCURLs)
		if err != nil {
			slog.Error("failed to connect to rpc providers", "error", err)
			os.Exit(1)
		}
		slog.Info("rpc providers connected", "count", len(providerList))
		go service.NewEvmListener(providerList, cfg.SafeBlockBuffer, cfg.UsdcListen, cfg.Network).Run(ctx)
	}

	<-ctx.Done()

}
