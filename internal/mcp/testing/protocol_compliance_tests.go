package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// ProtocolComplianceTestSuite validates MCP JSON-RPC 2.0 protocol compliance
type ProtocolComplianceTestSuite struct {
	factory   *MockClientFactory
	serverURL string
	logger    *logrus.Logger
	config    ComplianceTestConfig
	results   []ComplianceTestResult
}

type ComplianceTestConfig struct {
	StrictMode          bool          `json:"strict_mode"`
	TestTimeout         time.Duration `json:"test_timeout"`
	ValidateSchema      bool          `json:"validate_schema"`
	CheckErrorCodes     bool          `json:"check_error_codes"`
	ValidateCapabilities bool         `json:"validate_capabilities"`
	TestBatchRequests   bool          `json:"test_batch_requests"`
	TestNotifications   bool          `json:"test_notifications"`
}

type ComplianceTestResult struct {
	TestName        string                 `json:"test_name"`
	Category        ComplianceCategory     `json:"category"`
	Success         bool                   `json:"success"`
	Violations      []ComplianceViolation  `json:"violations"`
	Duration        time.Duration          `json:"duration"`
	RequestMessage  interface{}            `json:"request_message"`
	ResponseMessage interface{}            `json:"response_message"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type ComplianceCategory string

const (
	CategoryProtocol      ComplianceCategory = "protocol"
	CategorySchema        ComplianceCategory = "schema"
	CategoryCapabilities  ComplianceCategory = "capabilities"
	CategoryErrorHandling ComplianceCategory = "error_handling"
	CategoryTransport     ComplianceCategory = "transport"
	CategorySecurity      ComplianceCategory = "security"
)

type ComplianceViolation struct {
	Type        ViolationType `json:"type"`
	Severity    Severity      `json:"severity"`
	Message     string        `json:"message"`
	Location    string        `json:"location"`
	Expected    interface{}   `json:"expected,omitempty"`
	Actual      interface{}   `json:"actual,omitempty"`
	Reference   string        `json:"reference,omitempty"`
}

type ViolationType string

const (
	ViolationMissingField      ViolationType = "missing_field"
	ViolationInvalidType       ViolationType = "invalid_type"
	ViolationInvalidValue      ViolationType = "invalid_value"
	ViolationProtocolViolation ViolationType = "protocol_violation"
	ViolationSchemaViolation   ViolationType = "schema_violation"
	ViolationSecurityViolation ViolationType = "security_violation"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityMajor    Severity = "major"
	SeverityMinor    Severity = "minor"
	SeverityInfo     Severity = "info"
)

// JSON-RPC 2.0 message structures for validation
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc" validate:"required,eq=2.0"`
	ID      interface{} `json:"id,omitempty" validate:"omitempty"`
	Method  string      `json:"method" validate:"required"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc" validate:"required,eq=2.0"`
	ID      interface{} `json:"id" validate:"required"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc" validate:"required,eq=2.0"`
	Method  string      `json:"method" validate:"required"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code" validate:"required"`
	Message string      `json:"message" validate:"required"`
	Data    interface{} `json:"data,omitempty"`
}

type BatchRequest []JSONRPCRequest
type BatchResponse []JSONRPCResponse

// MCP-specific message types
type MCPInitializeRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  InitializeParams       `json:"params"`
}

type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
}

type SamplingCapability struct{}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type LoggingCapability struct{}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// JSON-RPC 2.0 error codes
const (
	ErrorParseError     = -32700
	ErrorInvalidRequest = -32600
	ErrorMethodNotFound = -32601
	ErrorInvalidParams  = -32602
	ErrorInternalError  = -32603
	// Server error range: -32099 to -32000
)

func NewProtocolComplianceTestSuite(serverURL string, config ComplianceTestConfig) *ProtocolComplianceTestSuite {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if config.TestTimeout == 0 {
		config.TestTimeout = 30 * time.Second
	}

	return &ProtocolComplianceTestSuite{
		factory:   NewMockClientFactory(),
		serverURL: serverURL,
		logger:    logger,
		config:    config,
		results:   make([]ComplianceTestResult, 0),
	}
}

func (suite *ProtocolComplianceTestSuite) RunProtocolComplianceTests(ctx context.Context, t *testing.T) {
	suite.logger.Info("Starting comprehensive protocol compliance tests")

	tests := []struct {
		name     string
		category ComplianceCategory
		fn       func(context.Context, *testing.T)
	}{
		{"TestJSONRPC2Protocol", CategoryProtocol, suite.TestJSONRPC2Protocol},
		{"TestMessageStructure", CategorySchema, suite.TestMessageStructure},
		{"TestErrorHandling", CategoryErrorHandling, suite.TestErrorHandling},
		{"TestCapabilityNegotiation", CategoryCapabilities, suite.TestCapabilityNegotiation},
		{"TestBatchRequests", CategoryProtocol, suite.TestBatchRequests},
		{"TestNotifications", CategoryProtocol, suite.TestNotifications},
		{"TestIDHandling", CategoryProtocol, suite.TestIDHandling},
		{"TestMCPSpecificCompliance", CategorySchema, suite.TestMCPSpecificCompliance},
		{"TestTransportCompliance", CategoryTransport, suite.TestTransportCompliance},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, suite.config.TestTimeout)
			defer cancel()
			
			suite.logger.WithFields(logrus.Fields{
				"test":     test.name,
				"category": test.category,
			}).Info("Running protocol compliance test")
			
			test.fn(testCtx, t)
		})
	}
}

func (suite *ProtocolComplianceTestSuite) TestJSONRPC2Protocol(ctx context.Context, t *testing.T) {
	result := ComplianceTestResult{
		TestName:   "JSONRPC2Protocol",
		Category:   CategoryProtocol,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("protocol_client")
	defer suite.factory.RemoveClient(client.ID)

	// Test JSON-RPC 2.0 version field
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	response, err := suite.sendRawRequest(ctx, client, request)
	if err != nil {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationProtocolViolation,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("Failed to send JSON-RPC request: %v", err),
		})
	} else {
		suite.validateJSONRPC2Response(response, &result)
	}

	// Test invalid JSON-RPC version
	invalidRequest := map[string]interface{}{
		"jsonrpc": "1.0", // Invalid version
		"id":      2,
		"method":  "tools/list",
	}

	_, err = suite.sendRawRequest(ctx, client, invalidRequest)
	if err == nil {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationProtocolViolation,
			Severity: SeverityMajor,
			Message:  "Server should reject requests with invalid JSON-RPC version",
			Expected: "Error response",
			Actual:   "Success response",
		})
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)

	assert.True(t, result.Success, "JSON-RPC 2.0 protocol compliance should pass")
}

func (suite *ProtocolComplianceTestSuite) TestMessageStructure(ctx context.Context, t *testing.T) {
	result := ComplianceTestResult{
		TestName:   "MessageStructure",
		Category:   CategorySchema,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("structure_client")
	defer suite.factory.RemoveClient(client.ID)

	testCases := []struct {
		name    string
		request map[string]interface{}
		expectError bool
	}{
		{
			name: "valid_request",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/list",
			},
			expectError: false,
		},
		{
			name: "missing_jsonrpc",
			request: map[string]interface{}{
				"id":     2,
				"method": "tools/list",
			},
			expectError: true,
		},
		{
			name: "missing_method",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      3,
			},
			expectError: true,
		},
		{
			name: "invalid_method_type",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      4,
				"method":  123, // Should be string
			},
			expectError: true,
		},
		{
			name: "valid_notification",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "notifications/initialized",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		response, err := suite.sendRawRequest(ctx, client, tc.request)
		
		if tc.expectError {
			if err == nil && !suite.isErrorResponse(response) {
				result.Violations = append(result.Violations, ComplianceViolation{
					Type:     ViolationSchemaViolation,
					Severity: SeverityMajor,
					Message:  fmt.Sprintf("Test case '%s': Expected error for malformed request", tc.name),
					Expected: "Error response",
					Actual:   "Success response",
				})
			}
		} else {
			if err != nil || suite.isErrorResponse(response) {
				result.Violations = append(result.Violations, ComplianceViolation{
					Type:     ViolationSchemaViolation,
					Severity: SeverityMajor,
					Message:  fmt.Sprintf("Test case '%s': Valid request should succeed", tc.name),
					Expected: "Success response",
					Actual:   fmt.Sprintf("Error: %v", err),
				})
			}
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestErrorHandling(ctx context.Context, t *testing.T) {
	result := ComplianceTestResult{
		TestName:   "ErrorHandling",
		Category:   CategoryErrorHandling,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("error_client")
	defer suite.factory.RemoveClient(client.ID)

	errorTests := []struct {
		name          string
		request       map[string]interface{}
		expectedError int
		description   string
	}{
		{
			name: "parse_error",
			request: map[string]interface{}{
				"invalid": "json",
			},
			expectedError: ErrorParseError,
			description:   "Malformed JSON should return parse error",
		},
		{
			name: "invalid_request",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				// Missing method and id
			},
			expectedError: ErrorInvalidRequest,
			description:   "Invalid request structure should return invalid request error",
		},
		{
			name: "method_not_found",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "nonexistent/method",
			},
			expectedError: ErrorMethodNotFound,
			description:   "Unknown method should return method not found error",
		},
		{
			name: "invalid_params",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "tools/call",
				"params": map[string]interface{}{
					"invalid_param": "value",
				},
			},
			expectedError: ErrorInvalidParams,
			description:   "Invalid parameters should return invalid params error",
		},
	}

	for _, test := range errorTests {
		response, err := suite.sendRawRequest(ctx, client, test.request)
		
		if err != nil {
			// Transport level error - might be expected for some tests
			continue
		}

		if !suite.isErrorResponse(response) {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityCritical,
				Message:  fmt.Sprintf("%s: Expected error response", test.description),
				Location: test.name,
				Expected: fmt.Sprintf("Error code %d", test.expectedError),
				Actual:   "Success response",
			})
			continue
		}

		errorCode := suite.extractErrorCode(response)
		if errorCode != test.expectedError {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("%s: Incorrect error code", test.description),
				Location: test.name,
				Expected: test.expectedError,
				Actual:   errorCode,
			})
		}

		// Validate error structure
		suite.validateErrorStructure(response, &result, test.name)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestCapabilityNegotiation(ctx context.Context, t *testing.T) {
	if !suite.config.ValidateCapabilities {
		t.Skip("Capability validation disabled")
		return
	}

	result := ComplianceTestResult{
		TestName:   "CapabilityNegotiation",
		Category:   CategoryCapabilities,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("capability_client")
	defer suite.factory.RemoveClient(client.ID)

	// Test MCP initialization
	initRequest := MCPInitializeRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities: ClientCapabilities{
				Experimental: make(map[string]interface{}),
			},
			ClientInfo: ClientInfo{
				Name:    "TestClient",
				Version: "1.0.0",
			},
		},
	}

	response, err := suite.sendRawRequest(ctx, client, initRequest)
	if err != nil {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationProtocolViolation,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("Initialize request failed: %v", err),
		})
	} else {
		suite.validateInitializeResponse(response, &result)
	}

	// Test capability advertisement
	suite.testCapabilityAdvertisement(ctx, client, &result)

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestBatchRequests(ctx context.Context, t *testing.T) {
	if !suite.config.TestBatchRequests {
		t.Skip("Batch request testing disabled")
		return
	}

	result := ComplianceTestResult{
		TestName:   "BatchRequests",
		Category:   CategoryProtocol,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("batch_client")
	defer suite.factory.RemoveClient(client.ID)

	// Test batch request
	batchRequest := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/list",
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "resources/list",
		},
		{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized", // Notification in batch
		},
	}

	response, err := suite.sendRawRequest(ctx, client, batchRequest)
	if err != nil {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationProtocolViolation,
			Severity: SeverityMajor,
			Message:  fmt.Sprintf("Batch request failed: %v", err),
		})
	} else {
		suite.validateBatchResponse(response, &result)
	}

	// Test empty batch
	emptyBatch := []map[string]interface{}{}
	_, err = suite.sendRawRequest(ctx, client, emptyBatch)
	if err == nil {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationProtocolViolation,
			Severity: SeverityMajor,
			Message:  "Empty batch should return invalid request error",
		})
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestNotifications(ctx context.Context, t *testing.T) {
	if !suite.config.TestNotifications {
		t.Skip("Notification testing disabled")
		return
	}

	result := ComplianceTestResult{
		TestName:   "Notifications",
		Category:   CategoryProtocol,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("notification_client")
	defer suite.factory.RemoveClient(client.ID)

	// Test notification (no id field)
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	response, _ := suite.sendRawRequest(ctx, client, notification)

	// Notifications should not generate responses (unless it's an error)
	if response != nil && !suite.isErrorResponse(response) {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationProtocolViolation,
			Severity: SeverityMajor,
			Message:  "Notification should not generate response",
			Expected: "No response",
			Actual:   "Response received",
		})
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestIDHandling(ctx context.Context, t *testing.T) {
	result := ComplianceTestResult{
		TestName:   "IDHandling",
		Category:   CategoryProtocol,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("id_client")
	defer suite.factory.RemoveClient(client.ID)

	// Test different ID types
	idTests := []struct {
		id          interface{}
		description string
	}{
		{1, "integer ID"},
		{"test", "string ID"},
		{nil, "null ID"},
	}

	for _, test := range idTests {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      test.id,
			"method":  "tools/list",
		}

		response, err := suite.sendRawRequest(ctx, client, request)
		if err != nil {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("Request with %s failed: %v", test.description, err),
			})
			continue
		}

		// Validate that response ID matches request ID
		responseID := suite.extractResponseID(response)
		if !suite.idsEqual(test.id, responseID) {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityCritical,
				Message:  fmt.Sprintf("Response ID mismatch for %s", test.description),
				Expected: test.id,
				Actual:   responseID,
			})
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestMCPSpecificCompliance(ctx context.Context, t *testing.T) {
	result := ComplianceTestResult{
		TestName:   "MCPSpecificCompliance",
		Category:   CategorySchema,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	client := suite.createTestClient("mcp_client")
	defer suite.factory.RemoveClient(client.ID)

	// Test MCP-specific method patterns
	mcpMethods := []string{
		"tools/list",
		"tools/call", 
		"resources/list",
		"resources/read",
		"prompts/list",
		"prompts/get",
	}

	for _, method := range mcpMethods {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  method,
		}

		response, err := suite.sendRawRequest(ctx, client, request)
		if err != nil {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("MCP method %s not supported: %v", method, err),
				Location: method,
			})
		} else if suite.isErrorResponse(response) && suite.extractErrorCode(response) == ErrorMethodNotFound {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("MCP method %s not found", method),
				Location: method,
			})
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

func (suite *ProtocolComplianceTestSuite) TestTransportCompliance(ctx context.Context, t *testing.T) {
	result := ComplianceTestResult{
		TestName:   "TransportCompliance",
		Category:   CategoryTransport,
		Violations: make([]ComplianceViolation, 0),
		Metadata:   make(map[string]interface{}),
	}
	startTime := time.Now()

	// Test different transport types
	transports := []TransportType{TransportWebSocket, TransportHTTP, TransportSSE}

	for _, transport := range transports {
		clientConfig := ClientConfig{
			ID: fmt.Sprintf("transport_%s_client", transport),
			Name: "TransportTestClient",
			Version: "1.0.0",
			Transport: transport,
		}

		client, err := suite.factory.CreateClient(clientConfig)
		if err != nil {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("Failed to create %s client: %v", transport, err),
				Location: string(transport),
			})
			continue
		}

		err = client.Connect(ctx, suite.serverURL)
		if err != nil {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("Failed to connect via %s: %v", transport, err),
				Location: string(transport),
			})
		} else {
			// Test basic operation
			_, err = client.ListTools(ctx)
			if err != nil {
				result.Violations = append(result.Violations, ComplianceViolation{
					Type:     ViolationProtocolViolation,
					Severity: SeverityMajor,
					Message:  fmt.Sprintf("Basic operation failed for %s: %v", transport, err),
					Location: string(transport),
				})
			}
			client.Disconnect()
		}

		suite.factory.RemoveClient(clientConfig.ID)
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Violations) == 0
	suite.results = append(suite.results, result)
}

// Helper methods

func (suite *ProtocolComplianceTestSuite) createTestClient(clientID string) *MockMCPClient {
	clientConfig := ClientConfig{
		ID: clientID,
		Name: "ComplianceTestClient",
		Version: "1.0.0",
		Transport: TransportWebSocket,
	}

	client, err := suite.factory.CreateClient(clientConfig)
	if err != nil {
		suite.logger.WithError(err).Error("Failed to create test client")
		return nil
	}

	err = client.Connect(context.Background(), suite.serverURL)
	if err != nil {
		suite.logger.WithError(err).Error("Failed to connect test client")
		suite.factory.RemoveClient(clientID)
		return nil
	}

	return client
}

func (suite *ProtocolComplianceTestSuite) sendRawRequest(ctx context.Context, client *MockMCPClient, request interface{}) (interface{}, error) {
	// This would need to be implemented based on the actual client implementation
	// For now, we'll simulate the behavior
	
	// Convert request to JSON-RPC format
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Simulate sending and receiving
	// In a real implementation, this would send the raw JSON and parse the response
	
	// For testing purposes, we'll create a mock response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"tools": []interface{}{},
		},
	}

	suite.logger.WithFields(logrus.Fields{
		"request":  string(jsonBytes),
		"response": response,
	}).Debug("Raw JSON-RPC exchange")

	return response, nil
}

func (suite *ProtocolComplianceTestSuite) validateJSONRPC2Response(response interface{}, result *ComplianceTestResult) {
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Response is not a JSON object",
		})
		return
	}

	// Validate jsonrpc field
	if jsonrpc, exists := responseMap["jsonrpc"]; !exists {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationMissingField,
			Severity: SeverityCritical,
			Message:  "Missing 'jsonrpc' field in response",
			Location: "jsonrpc",
		})
	} else if jsonrpc != "2.0" {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationInvalidValue,
			Severity: SeverityCritical,
			Message:  "Invalid 'jsonrpc' field value",
			Location: "jsonrpc",
			Expected: "2.0",
			Actual:   jsonrpc,
		})
	}

	// Validate id field exists
	if _, exists := responseMap["id"]; !exists {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationMissingField,
			Severity: SeverityCritical,
			Message:  "Missing 'id' field in response",
			Location: "id",
		})
	}

	// Validate result or error field exists (but not both)
	hasResult := responseMap["result"] != nil
	hasError := responseMap["error"] != nil

	if !hasResult && !hasError {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Response must have either 'result' or 'error' field",
		})
	}

	if hasResult && hasError {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Response cannot have both 'result' and 'error' fields",
		})
	}
}

func (suite *ProtocolComplianceTestSuite) isErrorResponse(response interface{}) bool {
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return false
	}
	
	return responseMap["error"] != nil
}

func (suite *ProtocolComplianceTestSuite) extractErrorCode(response interface{}) int {
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return 0
	}
	
	errorObj, ok := responseMap["error"].(map[string]interface{})
	if !ok {
		return 0
	}
	
	code, ok := errorObj["code"].(float64)
	if !ok {
		return 0
	}
	
	return int(code)
}

func (suite *ProtocolComplianceTestSuite) extractResponseID(response interface{}) interface{} {
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return nil
	}
	
	return responseMap["id"]
}

func (suite *ProtocolComplianceTestSuite) idsEqual(id1, id2 interface{}) bool {
	return reflect.DeepEqual(id1, id2)
}

func (suite *ProtocolComplianceTestSuite) validateErrorStructure(response interface{}, result *ComplianceTestResult, location string) {
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return
	}
	
	errorObj, ok := responseMap["error"].(map[string]interface{})
	if !ok {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Error field is not an object",
			Location: location,
		})
		return
	}
	
	// Validate required error fields
	if _, exists := errorObj["code"]; !exists {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationMissingField,
			Severity: SeverityCritical,
			Message:  "Missing 'code' field in error object",
			Location: fmt.Sprintf("%s.error.code", location),
		})
	}
	
	if _, exists := errorObj["message"]; !exists {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationMissingField,
			Severity: SeverityCritical,
			Message:  "Missing 'message' field in error object",
			Location: fmt.Sprintf("%s.error.message", location),
		})
	}
}

func (suite *ProtocolComplianceTestSuite) validateInitializeResponse(response interface{}, result *ComplianceTestResult) {
	// Validate initialize response structure
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Initialize response is not a JSON object",
		})
		return
	}

	resultObj, ok := responseMap["result"].(map[string]interface{})
	if !ok {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Initialize result is not a JSON object",
		})
		return
	}

	// Check required fields
	requiredFields := []string{"protocolVersion", "capabilities", "serverInfo"}
	for _, field := range requiredFields {
		if _, exists := resultObj[field]; !exists {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationMissingField,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("Missing '%s' field in initialize result", field),
				Location: fmt.Sprintf("result.%s", field),
			})
		}
	}
}

func (suite *ProtocolComplianceTestSuite) testCapabilityAdvertisement(ctx context.Context, client *MockMCPClient, result *ComplianceTestResult) {
	// Test that advertised capabilities are actually available
	capabilities := []string{"tools", "resources", "prompts"}
	
	for _, capability := range capabilities {
		var method string
		switch capability {
		case "tools":
			method = "tools/list"
		case "resources":
			method = "resources/list"
		case "prompts":
			method = "prompts/list"
		}
		
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  method,
		}
		
		response, err := suite.sendRawRequest(ctx, client, request)
		if err != nil || suite.isErrorResponse(response) {
			result.Violations = append(result.Violations, ComplianceViolation{
				Type:     ViolationProtocolViolation,
				Severity: SeverityMajor,
				Message:  fmt.Sprintf("Advertised capability '%s' is not functional", capability),
				Location: capability,
			})
		}
	}
}

func (suite *ProtocolComplianceTestSuite) validateBatchResponse(response interface{}, result *ComplianceTestResult) {
	responseArray, ok := response.([]interface{})
	if !ok {
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:     ViolationSchemaViolation,
			Severity: SeverityCritical,
			Message:  "Batch response is not an array",
		})
		return
	}

	// Validate each response in the batch
	for i, resp := range responseArray {
		suite.validateJSONRPC2Response(resp, result)
		
		// Update location for batch context
		for j := len(result.Violations) - 1; j >= 0; j-- {
			if result.Violations[j].Location == "" {
				result.Violations[j].Location = fmt.Sprintf("batch[%d]", i)
				break
			}
		}
	}
}

func (suite *ProtocolComplianceTestSuite) GetResults() []ComplianceTestResult {
	return suite.results
}

func (suite *ProtocolComplianceTestSuite) GenerateComplianceReport() map[string]interface{} {
	report := map[string]interface{}{
		"total_tests":      len(suite.results),
		"passed_tests":     0,
		"failed_tests":     0,
		"total_violations": 0,
		"violation_summary": make(map[ViolationType]int),
		"severity_summary":  make(map[Severity]int),
		"category_summary":  make(map[ComplianceCategory]int),
	}

	for _, result := range suite.results {
		if result.Success {
			report["passed_tests"] = report["passed_tests"].(int) + 1
		} else {
			report["failed_tests"] = report["failed_tests"].(int) + 1
		}

		report["total_violations"] = report["total_violations"].(int) + len(result.Violations)

		for _, violation := range result.Violations {
			vMap := report["violation_summary"].(map[ViolationType]int)
			vMap[violation.Type]++
			
			sMap := report["severity_summary"].(map[Severity]int)
			sMap[violation.Severity]++
		}

		cMap := report["category_summary"].(map[ComplianceCategory]int)
		cMap[result.Category]++
	}

	return report
}