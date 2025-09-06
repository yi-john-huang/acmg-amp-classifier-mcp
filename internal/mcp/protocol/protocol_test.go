package protocol

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
)

// TestProtocolCoreBasicOperations tests basic protocol core functionality
func TestProtocolCoreBasicOperations(t *testing.T) {
	logger, _ := test.NewNullLogger()
	core := NewProtocolCore(logger)

	// Test initialization
	if core == nil {
		t.Fatal("Failed to create protocol core")
	}

	// Test capability retrieval
	caps := core.GetCapabilities()
	if caps == nil {
		t.Fatal("Failed to get capabilities")
	}

	// Test client initialization
	clientID := "test-client"
	clientCaps := map[string]interface{}{
		"protocol": map[string]interface{}{
			"version": "2025-01-01",
		},
	}

	err := core.InitializeClient(clientID, clientCaps)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Test stats
	stats := core.GetStats()
	if stats["active_sessions"].(int) != 1 {
		t.Errorf("Expected 1 active session, got %v", stats["active_sessions"])
	}

	// Test cleanup
	core.CleanupClient(clientID)
	stats = core.GetStats()
	if stats["active_sessions"].(int) != 0 {
		t.Errorf("Expected 0 active sessions after cleanup, got %v", stats["active_sessions"])
	}
}

// TestJSONRPCMessageProcessing tests JSON-RPC message processing
func TestJSONRPCMessageProcessing(t *testing.T) {
	logger, _ := test.NewNullLogger()
	core := NewProtocolCore(logger)

	clientID := "test-client"
	
	// Initialize client
	err := core.InitializeClient(clientID, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Test valid JSON-RPC request
	request := JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
	}
	
	reqData, _ := json.Marshal(request)
	ctx := context.Background()
	
	respData, err := core.ProcessMessage(ctx, clientID, reqData)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	var response JSONRPC2Response
	err = json.Unmarshal(respData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC 2.0, got %s", response.JSONRPC)
	}

	if response.ID != float64(1) { // JSON unmarshaling converts numbers to float64
		t.Errorf("Expected ID 1, got %v", response.ID)
	}
}

// TestInvalidJSONRPCMessage tests handling of invalid JSON-RPC messages
func TestInvalidJSONRPCMessage(t *testing.T) {
	logger, _ := test.NewNullLogger()
	core := NewProtocolCore(logger)

	clientID := "test-client"
	ctx := context.Background()
	
	// Initialize client
	core.InitializeClient(clientID, map[string]interface{}{})

	// Test invalid JSON
	respData, err := core.ProcessMessage(ctx, clientID, []byte("invalid json"))
	if err != nil {
		t.Fatalf("ProcessMessage should not return error: %v", err)
	}

	var response JSONRPC2Response
	err = json.Unmarshal(respData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Error == nil || response.Error.Code != ParseError {
		t.Errorf("Expected parse error, got %v", response.Error)
	}

	// Test invalid JSON-RPC version
	request := JSONRPC2Request{
		JSONRPC: "1.0",
		Method:  "test",
		ID:      1,
	}
	
	reqData, _ := json.Marshal(request)
	respData, err = core.ProcessMessage(ctx, clientID, reqData)
	if err != nil {
		t.Fatalf("ProcessMessage should not return error: %v", err)
	}

	err = json.Unmarshal(respData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Error == nil || response.Error.Code != InvalidRequest {
		t.Errorf("Expected invalid request error, got %v", response.Error)
	}
}

// TestCapabilityNegotiation tests capability negotiation
func TestCapabilityNegotiation(t *testing.T) {
	logger, _ := test.NewNullLogger()
	capMgr := NewCapabilityManager(logger)

	// Test server capabilities
	serverCaps := capMgr.GetCapabilities()
	if serverCaps == nil {
		t.Fatal("Failed to get server capabilities")
	}

	// Test protocol version negotiation
	clientCaps := map[string]interface{}{
		"protocol": map[string]interface{}{
			"supportedVersions": []interface{}{"2025-01-01", "2024-11-05"},
		},
	}

	err := capMgr.NegotiateCapabilities("test-client", clientCaps)
	if err != nil {
		t.Fatalf("Capability negotiation failed: %v", err)
	}

	// Test incompatible version
	clientCaps["protocol"] = map[string]interface{}{
		"supportedVersions": []interface{}{"1.0.0"},
	}

	err = capMgr.NegotiateCapabilities("test-client-2", clientCaps)
	if err == nil {
		t.Error("Expected capability negotiation to fail with incompatible version")
	}
}

// TestSessionManager tests session management
func TestSessionManager(t *testing.T) {
	logger, _ := test.NewNullLogger()
	sessionMgr := NewSessionManager(logger)

	clientID := "test-client"
	capabilities := map[string]interface{}{
		"client": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	// Test session creation
	err := sessionMgr.CreateSession(clientID, capabilities)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test session retrieval
	session, exists := sessionMgr.GetSession(clientID)
	if !exists {
		t.Fatal("Session not found")
	}

	if session.ClientName != "test-client" {
		t.Errorf("Expected client name 'test-client', got %s", session.ClientName)
	}

	// Test activity update
	oldActivity := session.LastActivity
	time.Sleep(time.Millisecond) // Ensure time difference
	sessionMgr.UpdateClientActivity(clientID)
	
	session, _ = sessionMgr.GetSession(clientID)
	if !session.LastActivity.After(oldActivity) {
		t.Error("Activity timestamp was not updated")
	}

	// Test session cleanup
	sessionMgr.RemoveSession(clientID)
	_, exists = sessionMgr.GetSession(clientID)
	if exists {
		t.Error("Session should have been removed")
	}
}

// TestRateLimiter tests rate limiting functionality
func TestRateLimiter(t *testing.T) {
	logger, _ := test.NewNullLogger()
	rateLimiter := NewRateLimiter(logger)

	clientID := "test-client"
	
	// Initialize client
	rateLimiter.InitializeClient(clientID)

	// Test allowing requests within limits
	for i := 0; i < 5; i++ {
		if !rateLimiter.AllowRequest(clientID) {
			t.Errorf("Request %d should have been allowed", i+1)
		}
	}

	// Test burst limit (should eventually be blocked)
	blocked := false
	for i := 0; i < 20; i++ {
		if !rateLimiter.AllowRequest(clientID) {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Error("Rate limiter should have blocked requests after burst limit")
	}

	// Test stats
	stats := rateLimiter.GetStats()
	if stats["total_clients"].(int) != 1 {
		t.Errorf("Expected 1 client, got %v", stats["total_clients"])
	}
}

// TestMessageRouter tests message routing
func TestMessageRouter(t *testing.T) {
	logger, _ := test.NewNullLogger()
	router := NewMessageRouter(logger)

	// Test supported methods
	methods := router.GetSupportedMethods()
	if len(methods) == 0 {
		t.Error("Router should have some supported methods")
	}

	// Test initialize method
	ctx := context.Background()
	request := &JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
	}

	response := router.HandleRequest(ctx, request)
	if response.Error != nil {
		t.Errorf("Initialize request failed: %v", response.Error)
	}

	// Test unknown method
	request.Method = "unknown_method"
	response = router.HandleRequest(ctx, request)
	if response.Error == nil || response.Error.Code != MethodNotFound {
		t.Error("Unknown method should return method not found error")
	}
}

// TestErrorCodes tests JSON-RPC error code constants
func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"ParseError", ParseError, -32700},
		{"InvalidRequest", InvalidRequest, -32600},
		{"MethodNotFound", MethodNotFound, -32601},
		{"InvalidParams", InvalidParams, -32602},
		{"InternalError", InternalError, -32603},
		{"MCPUnauthorized", MCPUnauthorized, -32000},
		{"MCPRateLimited", MCPRateLimited, -32001},
		{"MCPResourceError", MCPResourceError, -32002},
		{"MCPToolError", MCPToolError, -32003},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("Expected %s to be %d, got %d", tt.name, tt.expected, tt.code)
			}
		})
	}
}