// Purpose    : Unit tests for inter-node coordination API functionality
// Context    : HTTP API for health status sharing and failover coordination in distributed pg-metako
// Constraints: Must test all coordination logic, HTTP handlers, and avoid blocking server operations
package coordination

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pg-metako/internal/config"
)

// Mock implementations for testing

type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Post(url, contentType string, body interface{}) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

type mockTimeProvider struct {
	currentTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Printf(format string, args ...interface{}) {
	m.messages = append(m.messages, format)
}

type mockJSONEncoder struct {
	encodeErr error
	data      []byte
}

func (m *mockJSONEncoder) Marshal(v interface{}) ([]byte, error) {
	if m.encodeErr != nil {
		return nil, m.encodeErr
	}
	if m.data != nil {
		return m.data, nil
	}
	return json.Marshal(v)
}

type mockJSONDecoder struct {
	decodeErr error
	data      interface{}
}

func (m *mockJSONDecoder) Decode(v interface{}) error {
	if m.decodeErr != nil {
		return m.decodeErr
	}
	if m.data != nil {
		// Simple copy for testing
		if nodeStatus, ok := m.data.(NodeStatus); ok {
			if target, ok := v.(*NodeStatus); ok {
				*target = nodeStatus
			}
		}
		if proposal, ok := m.data.(FailoverProposal); ok {
			if target, ok := v.(*FailoverProposal); ok {
				*target = proposal
			}
		}
		if vote, ok := m.data.(FailoverVote); ok {
			if target, ok := v.(*FailoverVote); ok {
				*target = vote
			}
		}
	}
	return nil
}

// Test helper functions

func createTestConfig() *config.Config {
	return &config.Config{
		Identity: config.NodeIdentity{
			NodeName:    "test-node",
			LocalDBHost: "localhost",
			LocalDBPort: 5432,
			APIHost:     "localhost",
			APIPort:     8080,
		},
		LocalDB: config.NodeConfig{
			Name:     "test-node",
			Host:     "localhost",
			Port:     5432,
			Role:     config.RoleMaster,
			Username: "test",
			Password: "test",
			Database: "test",
		},
		ClusterMembers: []config.ClusterMember{
			{NodeName: "test-node", APIHost: "localhost", APIPort: 8080, Role: config.RoleMaster},
			{NodeName: "node2", APIHost: "localhost", APIPort: 8081, Role: config.RoleSlave},
			{NodeName: "node3", APIHost: "localhost", APIPort: 8082, Role: config.RoleSlave},
		},
		Coordination: config.CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    2,
			LocalNodePreference:  0.8,
		},
	}
}

func createTestCoordinationAPI() (*CoordinationAPI, *mockHTTPClient, *mockTimeProvider, *mockLogger, *mockJSONDecoder, *mockJSONEncoder, *config.Config) {
	cfg := createTestConfig()

	httpClient := &mockHTTPClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		},
	}

	timeProvider := &mockTimeProvider{
		currentTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	logger := &mockLogger{}
	jsonDecoder := &mockJSONDecoder{}
	jsonEncoder := &mockJSONEncoder{}

	api := NewCoordinationAPI(cfg)

	return api, httpClient, timeProvider, logger, jsonDecoder, jsonEncoder, cfg
}

// Unit Tests

func TestNewCoordinationAPI(t *testing.T) {
	cfg := createTestConfig()

	api := NewCoordinationAPI(cfg)

	if api == nil {
		t.Fatal("CoordinationAPI should not be nil")
	}

	if api.config != cfg {
		t.Error("CoordinationAPI config should match provided config")
	}

	if api.client == nil {
		t.Error("CoordinationAPI client should not be nil")
	}

	if api.httpServer == nil {
		t.Error("CoordinationAPI httpServer should not be nil")
	}

	if api.localStatus.NodeName != cfg.Identity.NodeName {
		t.Errorf("Expected local status node name '%s', got '%s'", cfg.Identity.NodeName, api.localStatus.NodeName)
	}

	if api.localStatus.Role != string(cfg.LocalDB.Role) {
		t.Errorf("Expected local status role '%s', got '%s'", string(cfg.LocalDB.Role), api.localStatus.Role)
	}

	if !api.localStatus.IsHealthy {
		t.Error("Local status should be healthy initially")
	}

	if api.clusterState.Nodes == nil {
		t.Error("Cluster state nodes should be initialized")
	}
}

func TestCoordinationAPI_UpdateLocalStatus(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	// Test updating status
	api.UpdateLocalStatus(false, true)

	if api.localStatus.IsHealthy {
		t.Error("Local status should be unhealthy after update")
	}

	if !api.localStatus.LocalDBHealthy {
		t.Error("Local DB should be healthy after update")
	}

	// Test updating status again
	api.UpdateLocalStatus(true, false)

	if !api.localStatus.IsHealthy {
		t.Error("Local status should be healthy after second update")
	}

	if api.localStatus.LocalDBHealthy {
		t.Error("Local DB should be unhealthy after second update")
	}
}

func TestCoordinationAPI_GetClusterState(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	// Add some test data to cluster state
	testNode := NodeStatus{
		NodeName:       "test-remote",
		IsHealthy:      true,
		Role:           "slave",
		LastHeartbeat:  time.Now(),
		LocalDBHealthy: true,
		APIVersion:     "1.0.0",
	}

	api.clusterState.Nodes["test-remote"] = testNode
	api.clusterState.CurrentMaster = "test-master"

	clusterState := api.GetClusterState()

	if clusterState.CurrentMaster != "test-master" {
		t.Errorf("Expected current master 'test-master', got '%s'", clusterState.CurrentMaster)
	}

	if len(clusterState.Nodes) != 1 {
		t.Errorf("Expected 1 node in cluster state, got %d", len(clusterState.Nodes))
	}

	if node, exists := clusterState.Nodes["test-remote"]; !exists {
		t.Error("Expected test-remote node to exist in cluster state")
	} else {
		if node.NodeName != "test-remote" {
			t.Errorf("Expected node name 'test-remote', got '%s'", node.NodeName)
		}
		if !node.IsHealthy {
			t.Error("Expected node to be healthy")
		}
	}
}

func TestCoordinationAPI_ProposeFailover(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	tests := []struct {
		name          string
		failedNode    string
		newMasterNode string
		expectError   bool
	}{
		{
			name:          "valid failover proposal",
			failedNode:    "failed-node",
			newMasterNode: "new-master",
			expectError:   false,
		},
		{
			name:          "empty failed node",
			failedNode:    "",
			newMasterNode: "new-master",
			expectError:   false, // Implementation doesn't validate this
		},
		{
			name:          "empty new master node",
			failedNode:    "failed-node",
			newMasterNode: "",
			expectError:   false, // Implementation doesn't validate this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := api.ProposeFailover(ctx, tt.failedNode, tt.newMasterNode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestCoordinationAPI_StartStop(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test Start - should not block
	err := api.Start(ctx)
	if err != nil {
		t.Errorf("Start should not return error: %v", err)
	}

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Test Stop
	err = api.Stop(ctx)
	if err != nil {
		t.Errorf("Stop should not return error: %v", err)
	}
}

// HTTP Handler Tests

func TestCoordinationAPI_HandleHealthCheck(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid POST request",
			method:         http.MethodPost,
			body:           `{"node_name":"remote-node","is_healthy":true,"role":"slave","last_heartbeat":"2023-01-01T12:00:00Z","local_db_healthy":true,"api_version":"1.0.0"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid JSON",
			method:         http.MethodPost,
			body:           `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			api.handleHealthCheck(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				// Check that response contains local status
				var response NodeStatus
				err := json.NewDecoder(rr.Body).Decode(&response)
				if err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if response.NodeName != api.localStatus.NodeName {
					t.Errorf("Expected response node name '%s', got '%s'", api.localStatus.NodeName, response.NodeName)
				}
			}
		})
	}
}

func TestCoordinationAPI_HandleStatusRequest(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "valid GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/status", nil)
			rr := httptest.NewRecorder()

			api.handleStatusRequest(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response NodeStatus
				err := json.NewDecoder(rr.Body).Decode(&response)
				if err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if response.NodeName != api.localStatus.NodeName {
					t.Errorf("Expected response node name '%s', got '%s'", api.localStatus.NodeName, response.NodeName)
				}
			}
		})
	}
}

func TestCoordinationAPI_HandleClusterState(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "valid GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/cluster", nil)
			rr := httptest.NewRecorder()

			api.handleClusterState(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response ClusterState
				err := json.NewDecoder(rr.Body).Decode(&response)
				if err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if response.Nodes == nil {
					t.Error("Expected cluster state nodes to be initialized")
				}
			}
		})
	}
}

func TestCoordinationAPI_HandleFailoverProposal(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid POST request",
			method:         http.MethodPost,
			body:           `{"proposer_node":"proposer","failed_node":"failed","new_master_node":"new-master","proposal_time":"2023-01-01T12:00:00Z","required_votes":2}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid JSON",
			method:         http.MethodPost,
			body:           `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/failover/propose", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			api.handleFailoverProposal(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				// Check that response contains a vote
				var response FailoverVote
				err := json.NewDecoder(rr.Body).Decode(&response)
				if err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				if response.VoterNode != api.config.Identity.NodeName {
					t.Errorf("Expected voter node '%s', got '%s'", api.config.Identity.NodeName, response.VoterNode)
				}

				if !response.Vote {
					t.Error("Expected vote to be true (automatic approval)")
				}
			}
		})
	}
}

func TestCoordinationAPI_HandleFailoverVote(t *testing.T) {
	api, _, _, _, _, _, _ := createTestCoordinationAPI()

	tests := []struct {
		name           string
		method         string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid POST request",
			method:         http.MethodPost,
			body:           `{"voter_node":"voter","proposal_id":"proposal-123","vote":true,"vote_time":"2023-01-01T12:00:00Z","reason":"test"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid JSON",
			method:         http.MethodPost,
			body:           `{"invalid": json}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/failover/vote", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			api.handleFailoverVote(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

// Data Structure Tests

func TestNodeStatus_Structure(t *testing.T) {
	now := time.Now()
	status := NodeStatus{
		NodeName:       "test-node",
		IsHealthy:      true,
		Role:           "master",
		LastHeartbeat:  now,
		LocalDBHealthy: true,
		APIVersion:     "1.0.0",
	}

	if status.NodeName != "test-node" {
		t.Errorf("Expected NodeName 'test-node', got '%s'", status.NodeName)
	}

	if !status.IsHealthy {
		t.Error("Expected IsHealthy to be true")
	}

	if status.Role != "master" {
		t.Errorf("Expected Role 'master', got '%s'", status.Role)
	}

	if !status.LastHeartbeat.Equal(now) {
		t.Errorf("Expected LastHeartbeat %v, got %v", now, status.LastHeartbeat)
	}

	if !status.LocalDBHealthy {
		t.Error("Expected LocalDBHealthy to be true")
	}

	if status.APIVersion != "1.0.0" {
		t.Errorf("Expected APIVersion '1.0.0', got '%s'", status.APIVersion)
	}
}

func TestFailoverProposal_Structure(t *testing.T) {
	now := time.Now()
	proposal := FailoverProposal{
		ProposerNode:  "proposer",
		FailedNode:    "failed",
		NewMasterNode: "new-master",
		ProposalTime:  now,
		RequiredVotes: 3,
	}

	if proposal.ProposerNode != "proposer" {
		t.Errorf("Expected ProposerNode 'proposer', got '%s'", proposal.ProposerNode)
	}

	if proposal.FailedNode != "failed" {
		t.Errorf("Expected FailedNode 'failed', got '%s'", proposal.FailedNode)
	}

	if proposal.NewMasterNode != "new-master" {
		t.Errorf("Expected NewMasterNode 'new-master', got '%s'", proposal.NewMasterNode)
	}

	if !proposal.ProposalTime.Equal(now) {
		t.Errorf("Expected ProposalTime %v, got %v", now, proposal.ProposalTime)
	}

	if proposal.RequiredVotes != 3 {
		t.Errorf("Expected RequiredVotes 3, got %d", proposal.RequiredVotes)
	}
}

func TestFailoverVote_Structure(t *testing.T) {
	now := time.Now()
	vote := FailoverVote{
		VoterNode:  "voter",
		ProposalID: "proposal-123",
		Vote:       true,
		VoteTime:   now,
		Reason:     "test reason",
	}

	if vote.VoterNode != "voter" {
		t.Errorf("Expected VoterNode 'voter', got '%s'", vote.VoterNode)
	}

	if vote.ProposalID != "proposal-123" {
		t.Errorf("Expected ProposalID 'proposal-123', got '%s'", vote.ProposalID)
	}

	if !vote.Vote {
		t.Error("Expected Vote to be true")
	}

	if !vote.VoteTime.Equal(now) {
		t.Errorf("Expected VoteTime %v, got %v", now, vote.VoteTime)
	}

	if vote.Reason != "test reason" {
		t.Errorf("Expected Reason 'test reason', got '%s'", vote.Reason)
	}
}

func TestClusterState_Structure(t *testing.T) {
	now := time.Now()
	nodes := make(map[string]NodeStatus)
	nodes["node1"] = NodeStatus{NodeName: "node1", IsHealthy: true}

	proposals := []FailoverProposal{
		{ProposerNode: "proposer", FailedNode: "failed", NewMasterNode: "new-master"},
	}

	state := ClusterState{
		Nodes:           nodes,
		CurrentMaster:   "master-node",
		LastUpdate:      now,
		ActiveProposals: proposals,
	}

	if len(state.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(state.Nodes))
	}

	if state.CurrentMaster != "master-node" {
		t.Errorf("Expected CurrentMaster 'master-node', got '%s'", state.CurrentMaster)
	}

	if !state.LastUpdate.Equal(now) {
		t.Errorf("Expected LastUpdate %v, got %v", now, state.LastUpdate)
	}

	if len(state.ActiveProposals) != 1 {
		t.Errorf("Expected 1 active proposal, got %d", len(state.ActiveProposals))
	}
}
