// Purpose    : Inter-node communication API for distributed pg-metako coordination
// Context    : HTTP API for health status sharing and failover coordination
// Constraints: Must be lightweight, reliable, and support consensus mechanisms

package coordination

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/logger"
)

// NodeStatus represents the status of a node in the cluster
type NodeStatus struct {
	NodeName       string    `json:"node_name"`
	IsHealthy      bool      `json:"is_healthy"`
	Role           string    `json:"role"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
	LocalDBHealthy bool      `json:"local_db_healthy"`
	APIVersion     string    `json:"api_version"`
}

// FailoverProposal represents a proposal for failover
type FailoverProposal struct {
	ProposerNode  string    `json:"proposer_node"`
	FailedNode    string    `json:"failed_node"`
	NewMasterNode string    `json:"new_master_node"`
	ProposalTime  time.Time `json:"proposal_time"`
	RequiredVotes int       `json:"required_votes"`
}

// FailoverVote represents a vote on a failover proposal
type FailoverVote struct {
	VoterNode  string    `json:"voter_node"`
	ProposalID string    `json:"proposal_id"`
	Vote       bool      `json:"vote"` // true = approve, false = reject
	VoteTime   time.Time `json:"vote_time"`
	Reason     string    `json:"reason,omitempty"`
}

// ClusterState represents the overall state of the cluster
type ClusterState struct {
	Nodes           map[string]NodeStatus `json:"nodes"`
	CurrentMaster   string                `json:"current_master"`
	LastUpdate      time.Time             `json:"last_update"`
	ActiveProposals []FailoverProposal    `json:"active_proposals,omitempty"`
}

// CoordinationAPI handles inter-node communication
type CoordinationAPI struct {
	config     *config.DistributedConfig
	httpServer *http.Server
	client     *http.Client

	// Local state
	localStatus NodeStatus

	// Cluster state
	clusterState ClusterState
}

// NewCoordinationAPI creates a new coordination API instance
func NewCoordinationAPI(cfg *config.DistributedConfig) *CoordinationAPI {
	api := &CoordinationAPI{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Coordination.CommunicationTimeout,
		},
		localStatus: NodeStatus{
			NodeName:      cfg.Identity.NodeName,
			IsHealthy:     true,
			Role:          string(cfg.LocalDB.Role),
			LastHeartbeat: time.Now(),
			APIVersion:    "1.0.0",
		},
		clusterState: ClusterState{
			Nodes:      make(map[string]NodeStatus),
			LastUpdate: time.Now(),
		},
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.handleHealthCheck)
	mux.HandleFunc("/status", api.handleStatusRequest)
	mux.HandleFunc("/cluster", api.handleClusterState)
	mux.HandleFunc("/failover/propose", api.handleFailoverProposal)
	mux.HandleFunc("/failover/vote", api.handleFailoverVote)

	api.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Identity.APIHost, cfg.Identity.APIPort),
		Handler: mux,
	}

	return api
}

// Start starts the coordination API server
func (api *CoordinationAPI) Start(ctx context.Context) error {
	logger.Printf("Starting coordination API on %s", api.httpServer.Addr)

	go func() {
		if err := api.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("Coordination API server error: %v", err)
		}
	}()

	// Start heartbeat routine
	go api.heartbeatRoutine(ctx)

	return nil
}

// Stop stops the coordination API server
func (api *CoordinationAPI) Stop(ctx context.Context) error {
	logger.Printf("Stopping coordination API")
	return api.httpServer.Shutdown(ctx)
}

// UpdateLocalStatus updates the local node status
func (api *CoordinationAPI) UpdateLocalStatus(isHealthy, localDBHealthy bool) {
	api.localStatus.IsHealthy = isHealthy
	api.localStatus.LocalDBHealthy = localDBHealthy
	api.localStatus.LastHeartbeat = time.Now()
}

// GetClusterState returns the current cluster state
func (api *CoordinationAPI) GetClusterState() ClusterState {
	return api.clusterState
}

// ProposeFailover proposes a failover to the cluster
func (api *CoordinationAPI) ProposeFailover(failedNode, newMasterNode string) error {
	proposal := FailoverProposal{
		ProposerNode:  api.config.Identity.NodeName,
		FailedNode:    failedNode,
		NewMasterNode: newMasterNode,
		ProposalTime:  time.Now(),
		RequiredVotes: api.config.Coordination.MinConsensusNodes,
	}

	// Send proposal to all cluster members
	for _, member := range api.config.ClusterMembers {
		if member.NodeName == api.config.Identity.NodeName {
			continue // Skip self
		}

		go func(member config.ClusterMember) {
			if err := api.sendFailoverProposal(member, proposal); err != nil {
				logger.Printf("Failed to send failover proposal to %s: %v", member.NodeName, err)
			}
		}(member)
	}

	return nil
}

// heartbeatRoutine sends periodic heartbeats to cluster members
func (api *CoordinationAPI) heartbeatRoutine(ctx context.Context) {
	ticker := time.NewTicker(api.config.Coordination.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			api.sendHeartbeats()
		}
	}
}

// sendHeartbeats sends heartbeat to all cluster members
func (api *CoordinationAPI) sendHeartbeats() {
	for _, member := range api.config.ClusterMembers {
		if member.NodeName == api.config.Identity.NodeName {
			continue // Skip self
		}

		go func(member config.ClusterMember) {
			if err := api.sendHeartbeat(member); err != nil {
				logger.Printf("Failed to send heartbeat to %s: %v", member.NodeName, err)
				// Mark node as potentially unhealthy
				api.markNodeUnhealthy(member.NodeName)
			} else {
				// Mark node as healthy
				api.markNodeHealthy(member.NodeName)
			}
		}(member)
	}
}

// sendHeartbeat sends a heartbeat to a specific cluster member
func (api *CoordinationAPI) sendHeartbeat(member config.ClusterMember) error {
	url := fmt.Sprintf("http://%s:%d/health", member.APIHost, member.APIPort)

	statusJSON, err := json.Marshal(api.localStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	resp, err := api.client.Post(url, "application/json", bytes.NewBuffer(statusJSON))
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}

// sendFailoverProposal sends a failover proposal to a cluster member
func (api *CoordinationAPI) sendFailoverProposal(member config.ClusterMember, proposal FailoverProposal) error {
	url := fmt.Sprintf("http://%s:%d/failover/propose", member.APIHost, member.APIPort)

	proposalJSON, err := json.Marshal(proposal)
	if err != nil {
		return fmt.Errorf("failed to marshal proposal: %w", err)
	}

	resp, err := api.client.Post(url, "application/json", bytes.NewBuffer(proposalJSON))
	if err != nil {
		return fmt.Errorf("failed to send proposal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proposal failed with status: %d", resp.StatusCode)
	}

	return nil
}

// markNodeHealthy marks a node as healthy in cluster state
func (api *CoordinationAPI) markNodeHealthy(nodeName string) {
	if status, exists := api.clusterState.Nodes[nodeName]; exists {
		status.IsHealthy = true
		status.LastHeartbeat = time.Now()
		api.clusterState.Nodes[nodeName] = status
	}
}

// markNodeUnhealthy marks a node as unhealthy in cluster state
func (api *CoordinationAPI) markNodeUnhealthy(nodeName string) {
	if status, exists := api.clusterState.Nodes[nodeName]; exists {
		status.IsHealthy = false
		api.clusterState.Nodes[nodeName] = status
	}
}

// HTTP Handlers

// handleHealthCheck handles health check requests from other nodes
func (api *CoordinationAPI) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var remoteStatus NodeStatus
	if err := json.NewDecoder(r.Body).Decode(&remoteStatus); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update cluster state with remote node status
	api.clusterState.Nodes[remoteStatus.NodeName] = remoteStatus
	api.clusterState.LastUpdate = time.Now()

	// Respond with local status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.localStatus)
}

// handleStatusRequest handles status requests
func (api *CoordinationAPI) handleStatusRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.localStatus)
}

// handleClusterState handles cluster state requests
func (api *CoordinationAPI) handleClusterState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.clusterState)
}

// handleFailoverProposal handles failover proposals
func (api *CoordinationAPI) handleFailoverProposal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var proposal FailoverProposal
	if err := json.NewDecoder(r.Body).Decode(&proposal); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process the proposal (simplified - in real implementation, would validate and vote)
	logger.Printf("Received failover proposal from %s: %s -> %s",
		proposal.ProposerNode, proposal.FailedNode, proposal.NewMasterNode)

	// For now, automatically approve if the proposal makes sense
	vote := FailoverVote{
		VoterNode:  api.config.Identity.NodeName,
		ProposalID: fmt.Sprintf("%s-%d", proposal.ProposerNode, proposal.ProposalTime.Unix()),
		Vote:       true, // Simplified approval logic
		VoteTime:   time.Now(),
		Reason:     "Automatic approval",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vote)
}

// handleFailoverVote handles failover votes
func (api *CoordinationAPI) handleFailoverVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var vote FailoverVote
	if err := json.NewDecoder(r.Body).Decode(&vote); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	logger.Printf("Received failover vote from %s: %t", vote.VoterNode, vote.Vote)

	w.WriteHeader(http.StatusOK)
}
