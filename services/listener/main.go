package main

import (
	"context"
	"flag"
	"fmt"

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
		fmt.Println("Error loading config:", err)
		return
	}

	fmt.Println("Loaded config for network:", cfg.Network)

	// pass the RPC URLs to the providers package to establish connection
	providerList, err := providers.Connect(&cfg.RPCURLs)
	if err != nil {
		fmt.Println("Error connecting to providers:", err)
		return
	}

	service.ListenToBlocks(context.Background(), &providerList)

}
