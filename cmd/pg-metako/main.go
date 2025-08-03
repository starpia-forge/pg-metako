package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"pg-metako/internal/config"
	"pg-metako/internal/logger"
	"pg-metako/internal/metako"
)

const (
	defaultConfigPath = "configs/example.yaml"
	appName           = "pg-metako"
	version           = "1.0.0"
)

func main() {
	// Parse command line flags
	var (
		configPath  = flag.String("config", defaultConfigPath, "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s version %s\n", appName, version)
		os.Exit(0)
	}

	// Load configuration
	logger.Printf("Loading configuration from %s", *configPath)
	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Printf("Configuration loaded successfully with %d nodes", len(cfg.Nodes))

	// Create and run application
	app, err := metako.NewApplication(cfg)
	if err != nil {
		logger.Fatalf("Failed to initialize application: %v", err)
	}

	// Run the application with signal handling
	ctx := context.Background()
	if err := app.Run(ctx); err != nil {
		logger.Fatalf("Application error: %v", err)
	}
}
