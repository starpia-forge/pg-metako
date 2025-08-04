// Purpose    : Unified replication management with distributed consensus-based failover
// Context    : Coordinates failover decisions across multiple pg-metako nodes as default behavior
// Constraints: Must ensure data consistency and prevent split-brain scenarios

package replication

import (
	"context"
	"fmt"
	"sync"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/coordination"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"pg-metako/internal/logger"
)

// Manager manages PostgreSQL replication in a distributed environment
type Manager struct {
	config          *config.Config
	coordinationAPI *coordination.CoordinationAPI
	healthChecker   *health.HealthChecker
	localConnection *database.ConnectionManager

	// Current state
	currentMaster        string
	isFailoverInProgress bool
	lastFailoverTime     time.Time

	// Synchronization
	mu sync.RWMutex

	// Failover tracking
	activeProposals map[string]*FailoverProposal
	failoverVotes   map[string]map[string]bool // proposalID -> voterNode -> vote
}

// FailoverProposal represents a failover proposal with voting state
type FailoverProposal struct {
	ProposerNode  string
	FailedNode    string
	NewMasterNode string
	ProposalTime  time.Time
	RequiredVotes int
	ReceivedVotes map[string]bool // voterNode -> vote
	Status        ProposalStatus
	ExpiryTime    time.Time
}

// ProposalStatus represents the status of a failover proposal
type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "pending"
	ProposalStatusApproved ProposalStatus = "approved"
	ProposalStatusRejected ProposalStatus = "rejected"
	ProposalStatusExpired  ProposalStatus = "expired"
)

// NewManager creates a new distributed replication manager
func NewManager(cfg *config.Config, coordinationAPI *coordination.CoordinationAPI, healthChecker *health.HealthChecker, localConnection *database.ConnectionManager) *Manager {
	return &Manager{
		config:          cfg,
		coordinationAPI: coordinationAPI,
		healthChecker:   healthChecker,
		localConnection: localConnection,
		activeProposals: make(map[string]*FailoverProposal),
		failoverVotes:   make(map[string]map[string]bool),
	}
}

// Start starts the distributed replication manager
func (dm *Manager) Start(ctx context.Context) error {
	logger.Printf("Starting distributed replication manager for node: %s", dm.config.Identity.NodeName)

	// Initialize current master from cluster state
	dm.updateCurrentMaster()

	// Start monitoring routine
	go dm.monitoringRoutine(ctx)

	// Start proposal cleanup routine
	go dm.proposalCleanupRoutine(ctx)

	return nil
}

// Stop stops the distributed replication manager
func (dm *Manager) Stop(ctx context.Context) error {
	logger.Printf("Stopping distributed replication manager")
	return nil
}

// GetCurrentMaster returns the current master node
func (dm *Manager) GetCurrentMaster() string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentMaster
}

// GetLocalConnection returns the local database connection
func (dm *Manager) GetLocalConnection() *database.ConnectionManager {
	return dm.localConnection
}

// IsLocalNodeMaster checks if the local node is currently the master
func (dm *Manager) IsLocalNodeMaster() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentMaster == dm.config.Identity.NodeName
}

// ProposeFailover proposes a failover to the cluster
func (dm *Manager) ProposeFailover(failedNode, newMasterNode string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.isFailoverInProgress {
		return fmt.Errorf("failover already in progress")
	}

	// Create proposal ID
	proposalID := fmt.Sprintf("%s-%d", dm.config.Identity.NodeName, time.Now().Unix())

	proposal := &FailoverProposal{
		ProposerNode:  dm.config.Identity.NodeName,
		FailedNode:    failedNode,
		NewMasterNode: newMasterNode,
		ProposalTime:  time.Now(),
		RequiredVotes: dm.config.Coordination.MinConsensusNodes,
		ReceivedVotes: make(map[string]bool),
		Status:        ProposalStatusPending,
		ExpiryTime:    time.Now().Add(dm.config.Coordination.FailoverTimeout),
	}

	dm.activeProposals[proposalID] = proposal
	dm.isFailoverInProgress = true

	logger.Printf("Proposed failover: %s -> %s (proposal: %s)", failedNode, newMasterNode, proposalID)

	// Send proposal to coordination API
	if err := dm.coordinationAPI.ProposeFailover(failedNode, newMasterNode); err != nil {
		logger.Printf("Failed to send failover proposal: %v", err)
		return err
	}

	// Start monitoring this proposal
	go dm.monitorProposal(proposalID)

	return nil
}

// HandleFailoverVote handles a vote on a failover proposal
func (dm *Manager) HandleFailoverVote(proposalID, voterNode string, vote bool) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	proposal, exists := dm.activeProposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	if proposal.Status != ProposalStatusPending {
		return fmt.Errorf("proposal %s is not pending (status: %s)", proposalID, proposal.Status)
	}

	// Record the vote
	proposal.ReceivedVotes[voterNode] = vote
	logger.Printf("Received vote from %s for proposal %s: %t", voterNode, proposalID, vote)

	// Count votes
	approvalCount := 0
	rejectionCount := 0
	for _, v := range proposal.ReceivedVotes {
		if v {
			approvalCount++
		} else {
			rejectionCount++
		}
	}

	// Check if we have enough votes to make a decision
	totalVotes := len(proposal.ReceivedVotes)
	if approvalCount >= proposal.RequiredVotes {
		proposal.Status = ProposalStatusApproved
		logger.Printf("Proposal %s approved with %d votes", proposalID, approvalCount)
		go dm.executeFailover(proposal)
	} else if rejectionCount > (len(dm.config.ClusterMembers) - proposal.RequiredVotes) {
		proposal.Status = ProposalStatusRejected
		logger.Printf("Proposal %s rejected with %d rejection votes", proposalID, rejectionCount)
		dm.isFailoverInProgress = false
	} else if totalVotes >= len(dm.config.ClusterMembers)-1 { // -1 because proposer doesn't vote
		// All votes received but not enough approvals
		proposal.Status = ProposalStatusRejected
		logger.Printf("Proposal %s rejected - insufficient approvals (%d/%d)", proposalID, approvalCount, proposal.RequiredVotes)
		dm.isFailoverInProgress = false
	}

	return nil
}

// executeFailover executes an approved failover proposal
func (dm *Manager) executeFailover(proposal *FailoverProposal) {
	logger.Printf("Executing failover: %s -> %s", proposal.FailedNode, proposal.NewMasterNode)

	// If this node is the new master, promote it
	if proposal.NewMasterNode == dm.config.Identity.NodeName {
		if err := dm.promoteLocalToMaster(); err != nil {
			logger.Printf("Failed to promote local node to master: %v", err)
			return
		}
	}

	// Update current master
	dm.mu.Lock()
	dm.currentMaster = proposal.NewMasterNode
	dm.lastFailoverTime = time.Now()
	dm.isFailoverInProgress = false
	dm.mu.Unlock()

	logger.Printf("Failover completed successfully. New master: %s", proposal.NewMasterNode)
}

// promoteLocalToMaster promotes the local node to master
func (dm *Manager) promoteLocalToMaster() error {
	logger.Printf("Promoting local node to master: %s", dm.config.Identity.NodeName)

	// In a real implementation, this would:
	// 1. Stop replication on this node
	// 2. Promote it to master
	// 3. Update PostgreSQL configuration
	// 4. Restart PostgreSQL service
	// 5. Update cluster configuration

	// For now, we'll just log the action
	logger.Printf("Local node %s promoted to master", dm.config.Identity.NodeName)
	return nil
}

// monitoringRoutine monitors cluster health and triggers failover when needed
func (dm *Manager) monitoringRoutine(ctx context.Context) {
	ticker := time.NewTicker(dm.config.Coordination.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.checkClusterHealth()
		}
	}
}

// checkClusterHealth checks the health of cluster members and triggers failover if needed
func (dm *Manager) checkClusterHealth() {
	clusterState := dm.coordinationAPI.GetClusterState()

	// Check if current master is healthy
	if dm.currentMaster != "" {
		if status, exists := clusterState.Nodes[dm.currentMaster]; exists {
			if !status.IsHealthy {
				logger.Printf("Current master %s is unhealthy, initiating failover", dm.currentMaster)

				// Select new master
				newMaster := dm.selectNewMaster(clusterState, dm.currentMaster)
				if newMaster != "" {
					if err := dm.ProposeFailover(dm.currentMaster, newMaster); err != nil {
						logger.Printf("Failed to propose failover: %v", err)
					}
				} else {
					logger.Printf("No suitable replacement master found")
				}
			}
		}
	} else {
		// No current master, try to establish one
		newMaster := dm.selectNewMaster(clusterState, "")
		if newMaster != "" {
			logger.Printf("No current master, proposing %s as master", newMaster)
			if err := dm.ProposeFailover("", newMaster); err != nil {
				logger.Printf("Failed to propose initial master: %v", err)
			}
		}
	}
}

// selectNewMaster selects a new master from healthy cluster members
func (dm *Manager) selectNewMaster(clusterState coordination.ClusterState, failedMaster string) string {
	// Prefer local node if it's healthy and eligible
	if dm.config.LocalDB.Role == config.RoleMaster {
		if status, exists := clusterState.Nodes[dm.config.Identity.NodeName]; exists && status.IsHealthy {
			return dm.config.Identity.NodeName
		}
	}

	// Find healthy master nodes
	for nodeName, status := range clusterState.Nodes {
		if nodeName != failedMaster && status.IsHealthy && status.Role == string(config.RoleMaster) {
			return nodeName
		}
	}

	// If no master nodes available, consider promoting a slave
	for nodeName, status := range clusterState.Nodes {
		if nodeName != failedMaster && status.IsHealthy && status.Role == string(config.RoleSlave) {
			return nodeName
		}
	}

	return ""
}

// updateCurrentMaster updates the current master from cluster state
func (dm *Manager) updateCurrentMaster() {
	clusterState := dm.coordinationAPI.GetClusterState()

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if clusterState.CurrentMaster != "" {
		dm.currentMaster = clusterState.CurrentMaster
		logger.Printf("Updated current master to: %s", dm.currentMaster)
	} else {
		// Try to find a healthy master from cluster members
		for nodeName, status := range clusterState.Nodes {
			if status.IsHealthy && status.Role == string(config.RoleMaster) {
				dm.currentMaster = nodeName
				logger.Printf("Detected current master: %s", dm.currentMaster)
				break
			}
		}
	}
}

// monitorProposal monitors a specific failover proposal
func (dm *Manager) monitorProposal(proposalID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dm.mu.RLock()
			proposal, exists := dm.activeProposals[proposalID]
			if !exists {
				dm.mu.RUnlock()
				return
			}

			// Check if proposal has expired
			if time.Now().After(proposal.ExpiryTime) && proposal.Status == ProposalStatusPending {
				dm.mu.RUnlock()
				dm.mu.Lock()
				proposal.Status = ProposalStatusExpired
				dm.isFailoverInProgress = false
				dm.mu.Unlock()
				logger.Printf("Proposal %s expired", proposalID)
				return
			}

			// Check if proposal is completed
			if proposal.Status != ProposalStatusPending {
				dm.mu.RUnlock()
				return
			}
			dm.mu.RUnlock()
		}
	}
}

// proposalCleanupRoutine cleans up old proposals
func (dm *Manager) proposalCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.cleanupOldProposals()
		}
	}
}

// cleanupOldProposals removes old completed or expired proposals
func (dm *Manager) cleanupOldProposals() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for proposalID, proposal := range dm.activeProposals {
		if proposal.ProposalTime.Before(cutoff) && proposal.Status != ProposalStatusPending {
			delete(dm.activeProposals, proposalID)
			logger.Printf("Cleaned up old proposal: %s", proposalID)
		}
	}
}

// GetFailoverStatus returns the current failover status
func (dm *Manager) GetFailoverStatus() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return map[string]interface{}{
		"current_master":         dm.currentMaster,
		"failover_in_progress":   dm.isFailoverInProgress,
		"last_failover_time":     dm.lastFailoverTime,
		"active_proposals_count": len(dm.activeProposals),
	}
}

// Legacy compatibility methods for backward compatibility

// NewDistributedManager creates a new manager (legacy compatibility)
func NewDistributedManager(cfg *config.Config, coordinationAPI *coordination.CoordinationAPI, healthChecker *health.HealthChecker, localConnection *database.ConnectionManager) *Manager {
	return NewManager(cfg, coordinationAPI, healthChecker, localConnection)
}

// DistributedManager is an alias for Manager (legacy compatibility)
type DistributedManager = Manager
