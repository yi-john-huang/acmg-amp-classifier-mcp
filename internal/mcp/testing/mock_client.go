package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// MockMCPClient simulates an MCP client for testing purposes
type MockMCPClient struct {
	ID              string
	Name            string
	Version         string
	Capabilities    []string
	Transport       TransportType
	ServerURL       string
	Connected       bool
	logger          *logrus.Logger
	responses       chan MCPResponse
	requests        chan MCPRequest
	conn            interface{} // *websocket.Conn or io.ReadWriteCloser
	mutex           sync.RWMutex
	requestID       int64
	callbacks       map[int64]func(MCPResponse)
	subscriptions   map[string]func(MCPNotification)
	connectionStats ConnectionStats
	config          MockClientConfig
	errorSimulation ErrorSimulationConfig
}

type MockClientConfig struct {
	ConnectTimeout    time.Duration `json:"connect_timeout"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	KeepAlive         bool          `json:"keep_alive"`
	EnableLogging     bool          `json:"enable_logging"`
	AutoReconnect     bool          `json:"auto_reconnect"`
	BatchRequests     bool          `json:"batch_requests"`
	MaxBatchSize      int           `json:"max_batch_size"`
	EnableCompression bool          `json:"enable_compression"`
}

type ErrorSimulationConfig struct {
	EnableErrorSim     bool    `json:"enable_error_simulation"`
	ConnectionFailRate float64 `json:"connection_fail_rate"`
	RequestFailRate    float64 `json:"request_fail_rate"`
	ResponseDelayMin   time.Duration `json:"response_delay_min"`
	ResponseDelayMax   time.Duration `json:"response_delay_max"`
	TimeoutRate        float64 `json:"timeout_rate"`
	MalformedRate      float64 `json:"malformed_rate"`
}

type ConnectionStats struct {
	ConnectedAt         time.Time     `json:"connected_at"`
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	BytesSent           int64         `json:"bytes_sent"`
	BytesReceived       int64         `json:"bytes_received"`
	ReconnectCount      int           `json:"reconnect_count"`
	LastError           string        `json:"last_error,omitempty"`
}

type TransportType string

const (
	TransportStdio     TransportType = "stdio"
	TransportWebSocket TransportType = "websocket"
	TransportHTTP      TransportType = "http"
	TransportSSE       TransportType = "sse"
)

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool invocation related structures
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type ToolCallResult struct {
	Content   []ContentBlock `json:"content"`
	IsError   bool           `json:"isError,omitempty"`
	Metadata  interface{}    `json:"_meta,omitempty"`
}

type ContentBlock struct {
	Type string      `json:"type"`
	Text string      `json:"text,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// Resource access structures
type ResourceRequest struct {
	URI string `json:"uri"`
}

type ResourceResponse struct {
	Contents []ResourceContent `json:"contents"`
	Metadata interface{}       `json:"_meta,omitempty"`
}

type ResourceContent struct {
	URI      string      `json:"uri"`
	MimeType string      `json:"mimeType,omitempty"`
	Text     string      `json:"text,omitempty"`
	Blob     []byte      `json:"blob,omitempty"`
	Metadata interface{} `json:"_meta,omitempty"`
}

func NewMockMCPClient(id, name, version string, transport TransportType) *MockMCPClient {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &MockMCPClient{
		ID:           id,
		Name:         name,
		Version:      version,
		Transport:    transport,
		logger:       logger,
		responses:    make(chan MCPResponse, 100),
		requests:     make(chan MCPRequest, 100),
		callbacks:    make(map[int64]func(MCPResponse)),
		subscriptions: make(map[string]func(MCPNotification)),
		config: MockClientConfig{
			ConnectTimeout:    10 * time.Second,
			RequestTimeout:    30 * time.Second,
			MaxRetries:        3,
			RetryDelay:        time.Second,
			KeepAlive:         true,
			EnableLogging:     true,
			AutoReconnect:     true,
			MaxBatchSize:      10,
			EnableCompression: false,
		},
	}
}

func (c *MockMCPClient) SetConfig(config MockClientConfig) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.config = config
	
	if !config.EnableLogging {
		c.logger.SetOutput(io.Discard)
	}
}

func (c *MockMCPClient) SetErrorSimulation(config ErrorSimulationConfig) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.errorSimulation = config
}

func (c *MockMCPClient) Connect(ctx context.Context, serverURL string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Connected {
		return fmt.Errorf("client %s is already connected", c.ID)
	}

	c.ServerURL = serverURL

	// Simulate error scenarios
	if c.errorSimulation.EnableErrorSim && c.shouldSimulateError(c.errorSimulation.ConnectionFailRate) {
		return fmt.Errorf("simulated connection failure")
	}

	switch c.Transport {
	case TransportWebSocket:
		return c.connectWebSocket(ctx)
	case TransportHTTP:
		return c.connectHTTP(ctx)
	case TransportStdio:
		return c.connectStdio(ctx)
	case TransportSSE:
		return c.connectSSE(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", c.Transport)
	}
}

func (c *MockMCPClient) connectWebSocket(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.ConnectTimeout)
	defer cancel()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, c.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	c.conn = conn
	c.Connected = true
	c.connectionStats.ConnectedAt = time.Now()

	go c.handleWebSocketMessages()
	
	c.logger.WithFields(logrus.Fields{
		"client_id": c.ID,
		"transport": c.Transport,
		"server":    c.ServerURL,
	}).Info("Mock MCP client connected")

	return nil
}

func (c *MockMCPClient) connectHTTP(ctx context.Context) error {
	// For HTTP transport, we don't maintain persistent connections
	// Connection is established per request
	c.Connected = true
	c.connectionStats.ConnectedAt = time.Now()

	c.logger.WithFields(logrus.Fields{
		"client_id": c.ID,
		"transport": c.Transport,
		"server":    c.ServerURL,
	}).Info("Mock MCP client configured for HTTP transport")

	return nil
}

func (c *MockMCPClient) connectStdio(ctx context.Context) error {
	// For testing, we'll simulate stdio with channels
	c.Connected = true
	c.connectionStats.ConnectedAt = time.Now()

	c.logger.WithFields(logrus.Fields{
		"client_id": c.ID,
		"transport": c.Transport,
	}).Info("Mock MCP client configured for stdio transport")

	return nil
}

func (c *MockMCPClient) connectSSE(ctx context.Context) error {
	// Server-Sent Events transport simulation
	c.Connected = true
	c.connectionStats.ConnectedAt = time.Now()

	c.logger.WithFields(logrus.Fields{
		"client_id": c.ID,
		"transport": c.Transport,
		"server":    c.ServerURL,
	}).Info("Mock MCP client configured for SSE transport")

	return nil
}

func (c *MockMCPClient) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.Connected {
		return nil
	}

	if c.conn != nil {
		if wsConn, ok := c.conn.(*websocket.Conn); ok {
			wsConn.Close()
		}
	}

	c.Connected = false
	c.conn = nil

	c.logger.WithField("client_id", c.ID).Info("Mock MCP client disconnected")
	return nil
}

func (c *MockMCPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolCallResult, error) {
	if !c.Connected {
		return nil, fmt.Errorf("client not connected")
	}

	// Simulate error scenarios
	if c.errorSimulation.EnableErrorSim && c.shouldSimulateError(c.errorSimulation.RequestFailRate) {
		return nil, fmt.Errorf("simulated tool call failure")
	}

	c.mutex.Lock()
	c.requestID++
	reqID := c.requestID
	c.mutex.Unlock()

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "tools/call",
		Params: ToolCallRequest{
			Name:      name,
			Arguments: arguments,
		},
	}

	response, err := c.sendRequestAndWait(ctx, request)
	if err != nil {
		c.connectionStats.FailedRequests++
		c.connectionStats.LastError = err.Error()
		return nil, err
	}

	c.connectionStats.SuccessfulRequests++

	if response.Error != nil {
		return nil, fmt.Errorf("tool call error: %s", response.Error.Message)
	}

	// Parse response into ToolCallResult
	result := &ToolCallResult{}
	if response.Result != nil {
		resultBytes, _ := json.Marshal(response.Result)
		json.Unmarshal(resultBytes, result)
	}

	c.logger.WithFields(logrus.Fields{
		"client_id": c.ID,
		"tool_name": name,
		"request_id": reqID,
	}).Debug("Tool call completed")

	return result, nil
}

func (c *MockMCPClient) GetResource(ctx context.Context, uri string) (*ResourceResponse, error) {
	if !c.Connected {
		return nil, fmt.Errorf("client not connected")
	}

	// Simulate error scenarios
	if c.errorSimulation.EnableErrorSim && c.shouldSimulateError(c.errorSimulation.RequestFailRate) {
		return nil, fmt.Errorf("simulated resource access failure")
	}

	c.mutex.Lock()
	c.requestID++
	reqID := c.requestID
	c.mutex.Unlock()

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "resources/read",
		Params: ResourceRequest{
			URI: uri,
		},
	}

	response, err := c.sendRequestAndWait(ctx, request)
	if err != nil {
		c.connectionStats.FailedRequests++
		return nil, err
	}

	c.connectionStats.SuccessfulRequests++

	if response.Error != nil {
		return nil, fmt.Errorf("resource access error: %s", response.Error.Message)
	}

	result := &ResourceResponse{}
	if response.Result != nil {
		resultBytes, _ := json.Marshal(response.Result)
		json.Unmarshal(resultBytes, result)
	}

	c.logger.WithFields(logrus.Fields{
		"client_id": c.ID,
		"resource_uri": uri,
		"request_id": reqID,
	}).Debug("Resource access completed")

	return result, nil
}

func (c *MockMCPClient) ListTools(ctx context.Context) ([]string, error) {
	if !c.Connected {
		return nil, fmt.Errorf("client not connected")
	}

	c.mutex.Lock()
	c.requestID++
	reqID := c.requestID
	c.mutex.Unlock()

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "tools/list",
	}

	response, err := c.sendRequestAndWait(ctx, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("list tools error: %s", response.Error.Message)
	}

	// Parse response - this would be server-specific
	var tools []string
	if response.Result != nil {
		if toolList, ok := response.Result.(map[string]interface{}); ok {
			if toolsArray, ok := toolList["tools"].([]interface{}); ok {
				for _, tool := range toolsArray {
					if toolMap, ok := tool.(map[string]interface{}); ok {
						if name, ok := toolMap["name"].(string); ok {
							tools = append(tools, name)
						}
					}
				}
			}
		}
	}

	return tools, nil
}

func (c *MockMCPClient) ListResources(ctx context.Context) ([]string, error) {
	if !c.Connected {
		return nil, fmt.Errorf("client not connected")
	}

	c.mutex.Lock()
	c.requestID++
	reqID := c.requestID
	c.mutex.Unlock()

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "resources/list",
	}

	response, err := c.sendRequestAndWait(ctx, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("list resources error: %s", response.Error.Message)
	}

	var resources []string
	if response.Result != nil {
		if resourceList, ok := response.Result.(map[string]interface{}); ok {
			if resourcesArray, ok := resourceList["resources"].([]interface{}); ok {
				for _, resource := range resourcesArray {
					if resourceMap, ok := resource.(map[string]interface{}); ok {
						if uri, ok := resourceMap["uri"].(string); ok {
							resources = append(resources, uri)
						}
					}
				}
			}
		}
	}

	return resources, nil
}

func (c *MockMCPClient) GetPrompt(ctx context.Context, name string, arguments map[string]interface{}) (string, error) {
	if !c.Connected {
		return "", fmt.Errorf("client not connected")
	}

	c.mutex.Lock()
	c.requestID++
	reqID := c.requestID
	c.mutex.Unlock()

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "prompts/get",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	response, err := c.sendRequestAndWait(ctx, request)
	if err != nil {
		return "", err
	}

	if response.Error != nil {
		return "", fmt.Errorf("get prompt error: %s", response.Error.Message)
	}

	// Extract prompt text from response
	if response.Result != nil {
		if promptData, ok := response.Result.(map[string]interface{}); ok {
			if messages, ok := promptData["messages"].([]interface{}); ok && len(messages) > 0 {
				if firstMsg, ok := messages[0].(map[string]interface{}); ok {
					if content, ok := firstMsg["content"].(map[string]interface{}); ok {
						if text, ok := content["text"].(string); ok {
							return text, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("failed to extract prompt text from response")
}

func (c *MockMCPClient) sendRequestAndWait(ctx context.Context, request MCPRequest) (MCPResponse, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		c.connectionStats.AverageResponseTime = (c.connectionStats.AverageResponseTime + duration) / 2
	}()

	c.connectionStats.TotalRequests++

	// Simulate response delay
	if c.errorSimulation.EnableErrorSim {
		delay := c.calculateResponseDelay()
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	// Simulate timeout
	if c.errorSimulation.EnableErrorSim && c.shouldSimulateError(c.errorSimulation.TimeoutRate) {
		return MCPResponse{}, fmt.Errorf("simulated request timeout")
	}

	// Create response channel for this specific request
	responseChan := make(chan MCPResponse, 1)
	
	c.mutex.Lock()
	c.callbacks[request.ID] = func(response MCPResponse) {
		responseChan <- response
	}
	c.mutex.Unlock()

	// Send request (implementation depends on transport)
	err := c.sendRequest(request)
	if err != nil {
		c.mutex.Lock()
		delete(c.callbacks, request.ID)
		c.mutex.Unlock()
		return MCPResponse{}, err
	}

	// Wait for response or timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	select {
	case response := <-responseChan:
		c.mutex.Lock()
		delete(c.callbacks, request.ID)
		c.mutex.Unlock()
		return response, nil
	case <-ctx.Done():
		c.mutex.Lock()
		delete(c.callbacks, request.ID)
		c.mutex.Unlock()
		return MCPResponse{}, ctx.Err()
	}
}

func (c *MockMCPClient) sendRequest(request MCPRequest) error {
	reqBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	c.connectionStats.BytesSent += int64(len(reqBytes))

	switch c.Transport {
	case TransportWebSocket:
		if wsConn, ok := c.conn.(*websocket.Conn); ok {
			return wsConn.WriteMessage(websocket.TextMessage, reqBytes)
		}
	case TransportHTTP:
		// For testing, we simulate HTTP requests
		return c.simulateHTTPRequest(reqBytes)
	case TransportStdio:
		// For testing, add to requests channel
		c.requests <- request
		return nil
	}

	return fmt.Errorf("unsupported transport for sending request")
}

func (c *MockMCPClient) simulateHTTPRequest(reqBytes []byte) error {
	// Simulate HTTP request processing
	if c.errorSimulation.EnableErrorSim && c.shouldSimulateError(c.errorSimulation.MalformedRate) {
		return fmt.Errorf("simulated HTTP request failure")
	}
	return nil
}

func (c *MockMCPClient) handleWebSocketMessages() {
	defer func() {
		if r := recover(); r != nil {
			c.logger.WithField("panic", r).Error("WebSocket message handler panic")
		}
	}()

	wsConn, ok := c.conn.(*websocket.Conn)
	if !ok {
		return
	}

	for c.Connected {
		_, message, err := wsConn.ReadMessage()
		if err != nil {
			if c.Connected {
				c.logger.WithError(err).Error("Failed to read WebSocket message")
				if c.config.AutoReconnect {
					go c.attemptReconnect()
				}
			}
			break
		}

		c.connectionStats.BytesReceived += int64(len(message))

		// Try to parse as response first
		var response MCPResponse
		if err := json.Unmarshal(message, &response); err == nil && response.ID != 0 {
			c.handleResponse(response)
			continue
		}

		// Try to parse as notification
		var notification MCPNotification
		if err := json.Unmarshal(message, &notification); err == nil {
			c.handleNotification(notification)
			continue
		}

		c.logger.WithField("message", string(message)).Warn("Failed to parse MCP message")
	}
}

func (c *MockMCPClient) handleResponse(response MCPResponse) {
	c.mutex.RLock()
	callback, exists := c.callbacks[response.ID]
	c.mutex.RUnlock()

	if exists && callback != nil {
		callback(response)
	}
}

func (c *MockMCPClient) handleNotification(notification MCPNotification) {
	c.mutex.RLock()
	handler, exists := c.subscriptions[notification.Method]
	c.mutex.RUnlock()

	if exists && handler != nil {
		handler(notification)
	}
}

func (c *MockMCPClient) attemptReconnect() {
	c.mutex.Lock()
	if !c.Connected {
		c.mutex.Unlock()
		return
	}
	c.Connected = false
	c.connectionStats.ReconnectCount++
	c.mutex.Unlock()

	for attempt := 1; attempt <= c.config.MaxRetries; attempt++ {
		c.logger.WithFields(logrus.Fields{
			"client_id": c.ID,
			"attempt":   attempt,
		}).Info("Attempting to reconnect")

		time.Sleep(c.config.RetryDelay * time.Duration(attempt))

		ctx, cancel := context.WithTimeout(context.Background(), c.config.ConnectTimeout)
		err := c.Connect(ctx, c.ServerURL)
		cancel()

		if err == nil {
			c.logger.WithField("client_id", c.ID).Info("Successfully reconnected")
			return
		}

		c.logger.WithError(err).WithFields(logrus.Fields{
			"client_id": c.ID,
			"attempt":   attempt,
		}).Warn("Reconnection attempt failed")
	}

	c.logger.WithField("client_id", c.ID).Error("Failed to reconnect after maximum attempts")
}

func (c *MockMCPClient) shouldSimulateError(rate float64) bool {
	return rate > 0 && (float64(time.Now().UnixNano()%1000)/1000.0) < rate
}

func (c *MockMCPClient) calculateResponseDelay() time.Duration {
	if c.errorSimulation.ResponseDelayMax == 0 {
		return 0
	}

	min := c.errorSimulation.ResponseDelayMin
	max := c.errorSimulation.ResponseDelayMax
	
	if min >= max {
		return min
	}

	range_ := max - min
	delay := min + time.Duration(time.Now().UnixNano()%int64(range_))
	return delay
}

func (c *MockMCPClient) GetStats() ConnectionStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connectionStats
}

func (c *MockMCPClient) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Connected
}

func (c *MockMCPClient) Subscribe(method string, handler func(MCPNotification)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.subscriptions[method] = handler
}

func (c *MockMCPClient) Unsubscribe(method string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.subscriptions, method)
}