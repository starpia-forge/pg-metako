package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"pg-metako/internal/replication"
	"pg-metako/internal/routing"
	"syscall"
	"time"
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
	log.Printf("Loading configuration from %s", *configPath)
	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully with %d nodes", len(cfg.Nodes))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	app, err := NewApplication(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start the application
	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	log.Printf("%s started successfully", appName)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping application...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Application stopped")
}

// Application represents the main application
type Application struct {
	config             *config.Config
	healthChecker      *health.HealthChecker
	replicationManager *replication.ReplicationManager
	queryRouter        *routing.QueryRouter
	connectionManagers []*database.ConnectionManager
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) (*Application, error) {
	// Initialize health checker
	healthChecker := health.NewHealthChecker(cfg.HealthCheck)

	// Initialize replication manager
	replicationManager := replication.NewReplicationManager(healthChecker)

	// Initialize query router
	queryRouter := routing.NewQueryRouter(replicationManager, cfg.LoadBalancer)

	// Create connection managers for all nodes
	var connectionManagers []*database.ConnectionManager
	for _, nodeConfig := range cfg.Nodes {
		connManager, err := database.NewConnectionManager(nodeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection manager for node %s: %w", nodeConfig.Name, err)
		}

		// Add to replication manager (which also adds to health checker)
		if err := replicationManager.AddNode(connManager); err != nil {
			return nil, fmt.Errorf("failed to add node %s to replication manager: %w", nodeConfig.Name, err)
		}

		connectionManagers = append(connectionManagers, connManager)
		log.Printf("Added node %s (%s) to cluster", nodeConfig.Name, nodeConfig.Role)
	}

	return &Application{
		config:             cfg,
		healthChecker:      healthChecker,
		replicationManager: replicationManager,
		queryRouter:        queryRouter,
		connectionManagers: connectionManagers,
	}, nil
}

// Start starts the application
func (app *Application) Start(ctx context.Context) error {
	log.Println("Starting application components...")

	// Connect to all database nodes
	for _, connManager := range app.connectionManagers {
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := connManager.Connect(connectCtx); err != nil {
			log.Printf("Warning: Failed to connect to node %s: %v", connManager.NodeName(), err)
		} else {
			log.Printf("Successfully connected to node %s", connManager.NodeName())
		}
		cancel()
	}

	// Start health monitoring
	if err := app.healthChecker.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start health monitoring: %w", err)
	}
	log.Println("Health monitoring started")

	// Start failover monitoring
	if err := app.replicationManager.StartFailoverMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start failover monitoring: %w", err)
	}
	log.Println("Failover monitoring started")

	// Start a goroutine to periodically log cluster status
	go app.statusReporter(ctx)

	return nil
}

// Stop stops the application gracefully
func (app *Application) Stop(ctx context.Context) error {
	log.Println("Stopping application components...")

	// Stop monitoring
	app.replicationManager.StopFailoverMonitoring()
	log.Println("Failover monitoring stopped")

	app.healthChecker.StopMonitoring()
	log.Println("Health monitoring stopped")

	// Close all database connections
	for _, connManager := range app.connectionManagers {
		if err := connManager.Close(); err != nil {
			log.Printf("Warning: Failed to close connection to node %s: %v", connManager.NodeName(), err)
		} else {
			log.Printf("Closed connection to node %s", connManager.NodeName())
		}
	}

	return nil
}

// statusReporter periodically reports cluster status
func (app *Application) statusReporter(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.logClusterStatus()
		}
	}
}

// logClusterStatus logs the current status of the cluster
func (app *Application) logClusterStatus() {
	// Get current master
	master := app.replicationManager.GetCurrentMaster()
	if master != nil {
		log.Printf("Current master: %s", master.NodeName())
	} else {
		log.Println("No current master available")
	}

	// Get node statuses
	statuses := app.replicationManager.GetAllNodeStatuses()
	healthyCount := 0
	for nodeName, status := range statuses {
		if status.IsHealthy {
			healthyCount++
		}
		log.Printf("Node %s: healthy=%t, failures=%d, last_checked=%v",
			nodeName, status.IsHealthy, status.FailureCount, status.LastChecked.Format(time.RFC3339))
	}

	// Get query router stats
	stats := app.queryRouter.GetConnectionStats()
	log.Printf("Query stats: total=%d, reads=%d, writes=%d, failed=%d",
		stats.TotalQueries, stats.ReadQueries, stats.WriteQueries, stats.FailedQueries)

	log.Printf("Cluster status: %d/%d nodes healthy", healthyCount, len(statuses))
}

// GetQueryRouter returns the query router for external use (e.g., HTTP API)
func (app *Application) GetQueryRouter() *routing.QueryRouter {
	return app.queryRouter
}

// GetReplicationManager returns the replication manager for external use
func (app *Application) GetReplicationManager() *replication.ReplicationManager {
	return app.replicationManager
}

// GetHealthChecker returns the health checker for external use
func (app *Application) GetHealthChecker() *health.HealthChecker {
	return app.healthChecker
}
