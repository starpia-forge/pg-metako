// Purpose    : Main application for distributed pg-metako deployment
// Context    : Manages distributed components with local node preference and coordination
// Constraints: Must handle distributed coordination, failover, and routing seamlessly

package metako

import (
	"context"
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

// Application represents the main distributed application
type Application struct {
	config             *config.Config
	healthChecker      *health.HealthChecker
	replicationManager *replication.DistributedManager
	coordinationAPI    *coordination.CoordinationAPI
	router             *routing.Router
	localConnection    *database.ConnectionManager
}

// NewApplication creates a new distributed application instance
func NewApplication(cfg *config.Config) (*Application, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize health checker
	healthChecker := health.NewHealthChecker(cfg.HealthCheck)

	// Create local database connection
	localConnection, err := database.NewConnectionManager(cfg.LocalDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create local connection: %w", err)
	}

	// Initialize coordination API
	coordinationAPI := coordination.NewCoordinationAPI(cfg)

	// Initialize distributed replication manager
	replicationManager := replication.NewDistributedManager(cfg, coordinationAPI, healthChecker, localConnection)

	// Initialize unified router
	router, err := routing.NewRouter(cfg, coordinationAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	logger.Printf("Initialized distributed application for node: %s", cfg.Identity.NodeName)
	logger.Printf("Local DB: %s (%s) at %s:%d", cfg.LocalDB.Name, cfg.LocalDB.Role, cfg.LocalDB.Host, cfg.LocalDB.Port)
	logger.Printf("Cluster members: %d", len(cfg.ClusterMembers))

	return &Application{
		config:             cfg,
		healthChecker:      healthChecker,
		replicationManager: replicationManager,
		coordinationAPI:    coordinationAPI,
		router:             router,
		localConnection:    localConnection,
	}, nil
}

// Start starts the distributed application
func (app *Application) Start(ctx context.Context) error {
	logger.Println("Starting distributed application components...")

	// Connect to local database
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if err := app.localConnection.Connect(connectCtx); err != nil {
		logger.Printf("Warning: Failed to connect to local database: %v", err)
	} else {
		logger.Printf("Successfully connected to local database: %s", app.config.LocalDB.Name)
	}
	cancel()

	// Initialize router
	if err := app.router.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize router: %w", err)
	}
	logger.Println("Router initialized")

	// Start coordination API
	if err := app.coordinationAPI.Start(ctx); err != nil {
		return fmt.Errorf("failed to start coordination API: %w", err)
	}
	logger.Println("Coordination API started")

	// Start distributed replication manager
	if err := app.replicationManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start replication manager: %w", err)
	}
	logger.Println("Distributed replication manager started")

	// Start health monitoring
	if err := app.healthChecker.StartMonitoring(ctx); err != nil {
		return fmt.Errorf("failed to start health monitoring: %w", err)
	}
	logger.Println("Health monitoring started")

	// Update remote connections
	if err := app.router.UpdateRemoteConnections(ctx); err != nil {
		logger.Printf("Warning: Failed to update remote connections: %v", err)
	}

	// Start a goroutine to periodically log cluster status
	go app.statusReporter(ctx)

	return nil
}

// Stop stops the application gracefully
func (app *Application) Stop(ctx context.Context) error {
	logger.Println("Stopping distributed application components...")

	// Stop distributed replication manager
	if err := app.replicationManager.Stop(ctx); err != nil {
		logger.Printf("Warning: Failed to stop replication manager: %v", err)
	}
	logger.Println("Distributed replication manager stopped")

	// Stop coordination API
	if err := app.coordinationAPI.Stop(ctx); err != nil {
		logger.Printf("Warning: Failed to stop coordination API: %v", err)
	}
	logger.Println("Coordination API stopped")

	// Stop health monitoring
	app.healthChecker.StopMonitoring()
	logger.Println("Health monitoring stopped")

	// Close router and all connections
	if err := app.router.Close(); err != nil {
		logger.Printf("Warning: Failed to close router: %v", err)
	}
	logger.Println("Router closed")

	// Close local connection
	if err := app.localConnection.Close(); err != nil {
		logger.Printf("Warning: Failed to close local connection: %v", err)
	} else {
		logger.Printf("Closed local database connection")
	}

	return nil
}

// Run runs the application with signal handling
func (app *Application) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the application
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Println("Application started successfully. Press Ctrl+C to stop.")

	// Wait for signal
	<-sigChan
	logger.Println("Received shutdown signal")

	// Cancel context to stop all goroutines
	cancel()

	// Stop the application with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()

	if err := app.Stop(stopCtx); err != nil {
		return fmt.Errorf("failed to stop application gracefully: %w", err)
	}

	logger.Println("Application stopped successfully")
	return nil
}

// statusReporter periodically logs cluster status
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

// logClusterStatus logs the current status of the distributed cluster
func (app *Application) logClusterStatus() {
	// Get current master from replication manager
	currentMaster := app.replicationManager.GetCurrentMaster()
	logger.Printf("Current master: %s", currentMaster)

	// Get cluster state from coordination API
	clusterState := app.coordinationAPI.GetClusterState()
	healthyCount := 0
	for nodeName, status := range clusterState.Nodes {
		if status.IsHealthy {
			healthyCount++
		}
		logger.Printf("Node %s: healthy=%t, role=%s, last_heartbeat=%v",
			nodeName, status.IsHealthy, status.Role, status.LastHeartbeat.Format(time.RFC3339))
	}

	// Get router stats
	stats := app.router.GetStats()
	logger.Printf("Router stats: total=%d, local=%d, remote=%d, reads=%d, writes=%d, failed=%d",
		stats.TotalQueries, stats.LocalQueries, stats.RemoteQueries,
		stats.ReadQueries, stats.WriteQueries, stats.FailedQueries)

	logger.Printf("Cluster summary: %d/%d nodes healthy, local_preference=%.1f",
		healthyCount, len(clusterState.Nodes), stats.LocalPreference)
}

// GetRouter returns the query router for external use
func (app *Application) GetRouter() *routing.Router {
	return app.router
}

// GetReplicationManager returns the replication manager for external use
func (app *Application) GetReplicationManager() *replication.DistributedManager {
	return app.replicationManager
}

// GetCoordinationAPI returns the coordination API for external use
func (app *Application) GetCoordinationAPI() *coordination.CoordinationAPI {
	return app.coordinationAPI
}

// GetLocalConnection returns the local database connection for external use
func (app *Application) GetLocalConnection() *database.ConnectionManager {
	return app.localConnection
}
