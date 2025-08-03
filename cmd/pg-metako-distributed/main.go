// Purpose    : Main application entry point for distributed pg-metako deployment
// Context    : Runs pg-metako with distributed configuration and local node preference
// Constraints: Must support both centralized and distributed configuration formats

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/coordination"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"pg-metako/internal/logger"
	"pg-metako/internal/replication"
	"pg-metako/internal/routing"
)

const (
	defaultConfigPath = "configs/distributed-node1.yaml"
	appName           = "pg-metako-distributed"
	version           = "1.0.0"
)

// DistributedApplication represents the distributed pg-metako application
type DistributedApplication struct {
	config             *config.DistributedConfig
	coordinationAPI    *coordination.CoordinationAPI
	distributedRouter  *routing.DistributedRouter
	replicationManager *replication.DistributedManager
	healthChecker      *health.HealthChecker
}

func main() {
	// Parse command line flags
	var (
		configPath  = flag.String("config", defaultConfigPath, "Path to distributed configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s version %s\n", appName, version)
		os.Exit(0)
	}

	// Load distributed configuration
	logger.Printf("Loading distributed configuration from %s", *configPath)
	cfg, err := config.LoadDistributedFromFile(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load distributed configuration: %v", err)
	}

	logger.Printf("Distributed configuration loaded successfully for node: %s", cfg.Identity.NodeName)

	// Create and run distributed application
	app, err := NewDistributedApplication(cfg)
	if err != nil {
		logger.Fatalf("Failed to initialize distributed application: %v", err)
	}

	// Run the application with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Printf("Shutdown signal received, stopping application...")
		cancel()
	}()

	if err := app.Run(ctx); err != nil {
		logger.Fatalf("Application error: %v", err)
	}

	logger.Printf("Application stopped gracefully")
}

// NewDistributedApplication creates a new distributed application instance
func NewDistributedApplication(cfg *config.DistributedConfig) (*DistributedApplication, error) {
	// Create coordination API
	coordinationAPI := coordination.NewCoordinationAPI(cfg)

	// Create health checker
	healthChecker := health.NewHealthChecker(cfg.HealthCheck)

	// Create local database connection
	localConnection, err := database.NewConnectionManager(cfg.LocalDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create local database connection: %w", err)
	}

	// Create distributed router
	distributedRouter, err := routing.NewDistributedRouter(cfg, coordinationAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to create distributed router: %w", err)
	}

	// Create distributed replication manager
	replicationManager := replication.NewDistributedManager(cfg, coordinationAPI, healthChecker, localConnection)

	return &DistributedApplication{
		config:             cfg,
		coordinationAPI:    coordinationAPI,
		distributedRouter:  distributedRouter,
		replicationManager: replicationManager,
		healthChecker:      healthChecker,
	}, nil
}

// Run starts the distributed application
func (app *DistributedApplication) Run(ctx context.Context) error {
	logger.Printf("Starting distributed pg-metako application for node: %s", app.config.Identity.NodeName)

	// Initialize distributed router
	if err := app.distributedRouter.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize distributed router: %w", err)
	}

	// Add local connection to health checker
	localConnection := app.distributedRouter.GetLocalConnection()
	if err := app.healthChecker.AddNode(localConnection); err != nil {
		return fmt.Errorf("failed to add local node to health checker: %w", err)
	}

	// Start coordination API
	if err := app.coordinationAPI.Start(ctx); err != nil {
		return fmt.Errorf("failed to start coordination API: %w", err)
	}

	// Start health monitoring
	if err := app.healthChecker.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start health monitoring: %w", err)
	}

	// Start replication manager
	if err := app.replicationManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start replication manager: %w", err)
	}

	logger.Printf("Distributed application started successfully")
	logger.Printf("Node: %s, Local DB: %s:%d, API: %s:%d",
		app.config.Identity.NodeName,
		app.config.LocalDB.Host, app.config.LocalDB.Port,
		app.config.Identity.APIHost, app.config.Identity.APIPort)

	// Start status reporting routine
	go app.statusReportingRoutine(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	return app.shutdown(ctx)
}

// shutdown gracefully shuts down the application
func (app *DistributedApplication) shutdown(ctx context.Context) error {
	logger.Printf("Shutting down distributed application...")

	// Stop replication manager
	if err := app.replicationManager.Stop(ctx); err != nil {
		logger.Printf("Error stopping replication manager: %v", err)
	}

	// Stop health monitoring
	app.healthChecker.StopMonitoring()

	// Stop coordination API
	if err := app.coordinationAPI.Stop(ctx); err != nil {
		logger.Printf("Error stopping coordination API: %v", err)
	}

	// Close distributed router
	if err := app.distributedRouter.Close(); err != nil {
		logger.Printf("Error closing distributed router: %v", err)
	}

	logger.Printf("Distributed application shutdown completed")
	return nil
}

// statusReportingRoutine periodically reports the status of the distributed application
func (app *DistributedApplication) statusReportingRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.reportStatus()
		}
	}
}

// reportStatus reports the current status of the distributed application
func (app *DistributedApplication) reportStatus() {
	// Get routing statistics
	routingStats := app.distributedRouter.GetStats()

	// Get cluster state
	clusterState := app.distributedRouter.GetClusterState()

	// Get failover status
	failoverStatus := app.replicationManager.GetFailoverStatus()

	logger.Printf("=== Distributed Node Status Report ===")
	logger.Printf("Node: %s", app.config.Identity.NodeName)
	logger.Printf("Local DB Role: %s", app.config.LocalDB.Role)
	logger.Printf("Current Master: %s", failoverStatus["current_master"])
	logger.Printf("Queries - Total: %d, Local: %d, Remote: %d",
		routingStats.TotalQueries, routingStats.LocalQueries, routingStats.RemoteQueries)
	logger.Printf("Local Preference Ratio: %.2f", routingStats.LocalPreference)
	logger.Printf("Cluster Nodes: %d", len(clusterState.Nodes))
	logger.Printf("Failover In Progress: %t", failoverStatus["failover_in_progress"])
	logger.Printf("=====================================")
}
