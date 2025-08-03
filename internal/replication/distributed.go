// Purpose    : Distributed replication management with consensus-based failover
// Context    : Coordinates failover decisions across multiple pg-metako nodes
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

// DistributedManager manages PostgreSQL replication in a distributed environment
type DistributedManager struct {
	config          *config.DistributedConfig
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

// NewDistributedManager creates a new distributed replication manager
func NewDistributedManager(cfg *config.DistributedConfig, coordinationAPI *coordination.CoordinationAPI, healthChecker *health.HealthChecker, localConnection *database.ConnectionManager) *DistributedManager {
	return &DistributedManager{
		config:          cfg,
		coordinationAPI: coordinationAPI,
		healthChecker:   healthChecker,
		localConnection: localConnection,
		activeProposals: make(map[string]*FailoverProposal),
		failoverVotes:   make(map[string]map[string]bool),
	}
}

// Start starts the distributed replication manager
func (dm *DistributedManager) Start(ctx context.Context) error {
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
func (dm *DistributedManager) Stop(ctx context.Context) error {
	logger.Printf("Stopping distributed replication manager")
	return nil
}

// GetCurrentMaster returns the current master node
func (dm *DistributedManager) GetCurrentMaster() string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentMaster
}

// GetLocalConnection returns the local database connection
func (dm *DistributedManager) GetLocalConnection() *database.ConnectionManager {
	return dm.localConnection
}

// IsLocalNodeMaster checks if the local node is currently the master
func (dm *DistributedManager) IsLocalNodeMaster() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentMaster == dm.config.Identity.NodeName
}

// ProposeFailover proposes a failover to the cluster
func (dm *DistributedManager) ProposeFailover(failedNode, newMasterNode string) error {
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

	logger.Printf("Proposing failover: %s -> %s (proposal: %s)", failedNode, newMasterNode, proposalID)

	// Send proposal to coordination API
	if err := dm.coordinationAPI.ProposeFailover(failedNode, newMasterNode); err != nil {
		delete(dm.activeProposals, proposalID)
		dm.isFailoverInProgress = false
		return fmt.Errorf("failed to send failover proposal: %w", err)
	}

	// Start monitoring this proposal
	go dm.monitorProposal(proposalID)

	return nil
}

// HandleFailoverVote handles a vote on a failover proposal
func (dm *DistributedManager) HandleFailoverVote(proposalID, voterNode string, vote bool) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	proposal, exists := dm.activeProposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	if proposal.Status != ProposalStatusPending {
		return fmt.Errorf("proposal %s is not pending", proposalID)
	}

	// Record the vote
	proposal.ReceivedVotes[voterNode] = vote

	logger.Printf("Received vote from %s for proposal %s: %t", voterNode, proposalID, vote)

	// Check if we have enough votes to make a decision
	approvalCount := 0
	rejectionCount := 0

	for _, v := range proposal.ReceivedVotes {
		if v {
			approvalCount++
		} else {
			rejectionCount++
		}
	}

	totalVotes := len(proposal.ReceivedVotes)

	// Check for approval (majority of required votes)
	if approvalCount >= proposal.RequiredVotes {
		proposal.Status = ProposalStatusApproved
		logger.Printf("Proposal %s approved with %d votes", proposalID, approvalCount)

		// Execute failover if we're the proposer
		if proposal.ProposerNode == dm.config.Identity.NodeName {
			go dm.executeFailover(proposal)
		}

		return nil
	}

	// Check for rejection with special handling for 2-node environments
	clusterSize := len(dm.config.ClusterMembers)

	// Special case for 2-node environments
	if clusterSize == 2 && proposal.RequiredVotes == 1 {
		// In 2-node setup with min_consensus_nodes=1, we need at least 1 approval
		// Only reject if we have explicit rejection from the only other available node
		// and no approvals, or if both nodes have voted and majority is rejection
		if totalVotes >= clusterSize {
			// All nodes have voted, use simple majority
			if rejectionCount > approvalCount {
				proposal.Status = ProposalStatusRejected
				logger.Printf("Proposal %s rejected in 2-node setup: %d rejections vs %d approvals",
					proposalID, rejectionCount, approvalCount)
				dm.isFailoverInProgress = false
				return nil
			}
		} else if rejectionCount >= clusterSize-1 && approvalCount == 0 {
			// All available nodes (excluding failed node) have rejected
			proposal.Status = ProposalStatusRejected
			logger.Printf("Proposal %s rejected in 2-node setup: all available nodes rejected", proposalID)
			dm.isFailoverInProgress = false
			return nil
		}
	} else {
		// Standard rejection logic for 3+ node environments
		if rejectionCount > (clusterSize - proposal.RequiredVotes) {
			proposal.Status = ProposalStatusRejected
			logger.Printf("Proposal %s rejected with %d rejection votes", proposalID, rejectionCount)
			dm.isFailoverInProgress = false
			return nil
		}
	}

	logger.Printf("Proposal %s still pending: %d approvals, %d rejections out of %d total votes",
		proposalID, approvalCount, rejectionCount, totalVotes)

	return nil
}

// executeFailover executes the approved failover
func (dm *DistributedManager) executeFailover(proposal *FailoverProposal) {
	logger.Printf("Executing failover: %s -> %s", proposal.FailedNode, proposal.NewMasterNode)

	// Update current master
	dm.mu.Lock()
	dm.currentMaster = proposal.NewMasterNode
	dm.lastFailoverTime = time.Now()
	dm.isFailoverInProgress = false
	dm.mu.Unlock()

	// If the new master is the local node, promote it
	if proposal.NewMasterNode == dm.config.Identity.NodeName {
		if err := dm.promoteLocalToMaster(); err != nil {
			logger.Printf("Failed to promote local node to master: %v", err)
			return
		}
	}

	// Update coordination API with new master
	dm.coordinationAPI.UpdateLocalStatus(true, dm.localConnection.IsHealthy(context.Background()))

	logger.Printf("Failover completed successfully: new master is %s", proposal.NewMasterNode)
}

// promoteLocalToMaster promotes the local node to master
func (dm *DistributedManager) promoteLocalToMaster() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Printf("Promoting local node %s to master", dm.config.Identity.NodeName)

	// Execute PostgreSQL promotion command
	// This is a simplified version - in reality, you'd need to execute pg_promote() or similar
	query := "SELECT pg_promote()"
	_, err := dm.localConnection.ExecuteQuery(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to promote local node: %w", err)
	}

	// Update local configuration role
	dm.config.LocalDB.Role = config.RoleMaster

	logger.Printf("Local node %s successfully promoted to master", dm.config.Identity.NodeName)
	return nil
}

// monitoringRoutine monitors cluster health and initiates failover when needed
func (dm *DistributedManager) monitoringRoutine(ctx context.Context) {
	ticker := time.NewTicker(dm.config.HealthCheck.Interval)
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

// checkClusterHealth checks the health of the cluster and initiates failover if needed
func (dm *DistributedManager) checkClusterHealth() {
	clusterState := dm.coordinationAPI.GetClusterState()

	// Check if current master is healthy
	currentMaster := dm.GetCurrentMaster()
	if currentMaster == "" {
		// No master identified, try to determine from cluster state
		dm.updateCurrentMaster()
		return
	}

	// Check master health
	if masterStatus, exists := clusterState.Nodes[currentMaster]; exists {
		if !masterStatus.IsHealthy || !masterStatus.LocalDBHealthy {
			logger.Printf("Master node %s appears unhealthy, considering failover", currentMaster)

			// Only propose failover if we're not already in progress and enough time has passed
			dm.mu.RLock()
			canPropose := !dm.isFailoverInProgress &&
				time.Since(dm.lastFailoverTime) > dm.config.Coordination.FailoverTimeout
			dm.mu.RUnlock()

			if canPropose {
				// Find a suitable replacement master
				newMaster := dm.selectNewMaster(clusterState, currentMaster)
				if newMaster != "" {
					if err := dm.ProposeFailover(currentMaster, newMaster); err != nil {
						logger.Printf("Failed to propose failover: %v", err)
					}
				}
			}
		}
	}
}

// selectNewMaster selects a new master from available healthy nodes
func (dm *DistributedManager) selectNewMaster(clusterState coordination.ClusterState, failedMaster string) string {
	// Prefer local node if it's healthy and eligible
	if dm.config.LocalDB.Role == config.RoleSlave &&
		dm.config.Identity.NodeName != failedMaster &&
		dm.localConnection.IsHealthy(context.Background()) {
		return dm.config.Identity.NodeName
	}

	// Otherwise, find the first healthy slave node
	for _, member := range dm.config.ClusterMembers {
		if member.NodeName == failedMaster {
			continue // Skip failed master
		}

		if member.Role == config.RoleSlave {
			if nodeStatus, exists := clusterState.Nodes[member.NodeName]; exists {
				if nodeStatus.IsHealthy && nodeStatus.LocalDBHealthy {
					return member.NodeName
				}
			}
		}
	}

	return ""
}

// updateCurrentMaster updates the current master from cluster state
func (dm *DistributedManager) updateCurrentMaster() {
	clusterState := dm.coordinationAPI.GetClusterState()

	// Try to find current master from cluster state
	if clusterState.CurrentMaster != "" {
		dm.mu.Lock()
		dm.currentMaster = clusterState.CurrentMaster
		dm.mu.Unlock()
		return
	}

	// If not set, try to determine from cluster members
	for _, member := range dm.config.ClusterMembers {
		if member.Role == config.RoleMaster {
			if nodeStatus, exists := clusterState.Nodes[member.NodeName]; exists && nodeStatus.IsHealthy {
				dm.mu.Lock()
				dm.currentMaster = member.NodeName
				dm.mu.Unlock()
				return
			}
		}
	}
}

// monitorProposal monitors a specific failover proposal
func (dm *DistributedManager) monitorProposal(proposalID string) {
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
				proposal.Status = ProposalStatusExpired
				dm.isFailoverInProgress = false
				logger.Printf("Proposal %s expired", proposalID)
			}

			// Stop monitoring if proposal is no longer pending
			if proposal.Status != ProposalStatusPending {
				dm.mu.RUnlock()
				return
			}
			dm.mu.RUnlock()
		}
	}
}

// proposalCleanupRoutine cleans up old proposals
func (dm *DistributedManager) proposalCleanupRoutine(ctx context.Context) {
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
func (dm *DistributedManager) cleanupOldProposals() {
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
func (dm *DistributedManager) GetFailoverStatus() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return map[string]interface{}{
		"current_master":         dm.currentMaster,
		"failover_in_progress":   dm.isFailoverInProgress,
		"last_failover_time":     dm.lastFailoverTime,
		"active_proposals_count": len(dm.activeProposals),
	}
}
