// Purpose    : Main application orchestration and lifecycle management
// Context    : PostgreSQL replication management system core application
// Constraints: Must handle graceful startup, shutdown, and component coordination

package metako

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"pg-metako/internal/replication"
	"pg-metako/internal/routing"
)

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
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

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

// Run runs the application with signal handling
func (app *Application) Run(ctx context.Context) error {
	// Start the application
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	log.Println("Application started successfully")

	// Wait for shutdown signal or context cancellation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		log.Println("Context cancelled, stopping application...")
	case <-sigChan:
		log.Println("Shutdown signal received, stopping application...")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
		return err
	}

	log.Println("Application stopped")
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
