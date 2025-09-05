package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// HTTPSSETransport implements MCP communication over HTTP with Server-Sent Events
type HTTPSSETransport struct {
	logger      *logrus.Logger
	server      *http.Server
	router      *gin.Engine
	host        string
	port        int
	clients     map[string]*SSEClient
	clientsMu   sync.RWMutex
	messagesCh  chan HTTPMessage
	closed      bool
	mu          sync.RWMutex
}

// SSEClient represents a connected MCP client via SSE
type SSEClient struct {
	ID       string
	Writer   gin.ResponseWriter
	Request  *http.Request
	Messages chan []byte
	Done     chan struct{}
	mu       sync.RWMutex
}

// HTTPMessage represents a message received via HTTP
type HTTPMessage struct {
	ClientID string
	Data     []byte
}

// NewHTTPSSETransport creates a new HTTP SSE transport for remote AI agents
func NewHTTPSSETransport(logger *logrus.Logger, host string, port int) *HTTPSSETransport {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	transport := &HTTPSSETransport{
		logger:     logger,
		router:     router,
		host:       host,
		port:       port,
		clients:    make(map[string]*SSEClient),
		messagesCh: make(chan HTTPMessage, 100),
	}

	// Set up routes
	transport.setupRoutes()

	return transport
}

// setupRoutes configures HTTP routes for MCP communication
func (h *HTTPSSETransport) setupRoutes() {
	// SSE endpoint for receiving messages from server
	h.router.GET("/mcp/sse", h.handleSSEConnection)
	
	// HTTP endpoint for sending messages to server
	h.router.POST("/mcp/message", h.handleMessage)
	
	// Health check endpoint
	h.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"transport": "http-sse",
			"clients":   len(h.clients),
		})
	})
}

// Start initializes the HTTP SSE transport
func (h *HTTPSSETransport) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.closed {
		return fmt.Errorf("transport is closed")
	}

	addr := fmt.Sprintf("%s:%d", h.host, h.port)
	h.server = &http.Server{
		Addr:    addr,
		Handler: h.router,
	}

	h.logger.WithFields(logrus.Fields{
		"address": addr,
		"type":    "http-sse",
	}).Info("Starting HTTP SSE transport for MCP communication")

	// Start server in goroutine
	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.WithError(err).Error("HTTP server failed")
		}
	}()

	// Start message processor
	go h.processMessages(ctx)

	return nil
}

// handleSSEConnection handles Server-Sent Events connections
func (h *HTTPSSETransport) handleSSEConnection(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id parameter required"})
		return
	}

	h.logger.WithField("client_id", clientID).Info("New SSE client connecting")

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Create client
	client := &SSEClient{
		ID:       clientID,
		Writer:   c.Writer,
		Request:  c.Request,
		Messages: make(chan []byte, 50),
		Done:     make(chan struct{}),
	}

	// Register client
	h.clientsMu.Lock()
	h.clients[clientID] = client
	h.clientsMu.Unlock()

	defer func() {
		// Unregister client
		h.clientsMu.Lock()
		delete(h.clients, clientID)
		h.clientsMu.Unlock()
		close(client.Done)
		h.logger.WithField("client_id", clientID).Info("SSE client disconnected")
	}()

	// Send keep-alive messages and handle client messages
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-client.Done:
			return
		case message := <-client.Messages:
			// Send message to client
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(message))
			c.Writer.Flush()
		case <-ticker.C:
			// Send keep-alive
			fmt.Fprintf(c.Writer, "data: {\"type\":\"ping\"}\n\n")
			c.Writer.Flush()
		}
	}
}

// handleMessage handles incoming HTTP messages
func (h *HTTPSSETransport) handleMessage(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id parameter required"})
		return
	}

	var message json.RawMessage
	if err := c.ShouldBindJSON(&message); err != nil {
		h.logger.WithError(err).Error("Failed to parse JSON message")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Queue message for processing
	select {
	case h.messagesCh <- HTTPMessage{ClientID: clientID, Data: message}:
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	default:
		h.logger.Error("Message queue full")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Message queue full"})
	}
}

// processMessages processes incoming HTTP messages
func (h *HTTPSSETransport) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-h.messagesCh:
			h.logger.WithFields(logrus.Fields{
				"client_id":      msg.ClientID,
				"message_length": len(msg.Data),
			}).Debug("Processing HTTP message")
			
			// TODO: Route message to MCP handler
			// For now, just log it
		}
	}
}

// ReadMessage reads a message from the HTTP transport
func (h *HTTPSSETransport) ReadMessage() ([]byte, error) {
	select {
	case msg := <-h.messagesCh:
		return msg.Data, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("read timeout")
	}
}

// WriteMessage sends a message to all connected clients
func (h *HTTPSSETransport) WriteMessage(message []byte) error {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	if len(h.clients) == 0 {
		return fmt.Errorf("no connected clients")
	}

	for clientID, client := range h.clients {
		select {
		case client.Messages <- message:
			h.logger.WithField("client_id", clientID).Debug("Message queued for client")
		default:
			h.logger.WithField("client_id", clientID).Warn("Client message queue full, dropping message")
		}
	}

	return nil
}

// WriteJSONMessage writes a JSON object as a message
func (h *HTTPSSETransport) WriteJSONMessage(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return h.WriteMessage(data)
}

// Close closes the HTTP SSE transport
func (h *HTTPSSETransport) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.closed {
		return nil
	}
	
	h.closed = true

	// Close all client connections
	h.clientsMu.Lock()
	for _, client := range h.clients {
		close(client.Done)
	}
	h.clients = make(map[string]*SSEClient)
	h.clientsMu.Unlock()

	// Shutdown HTTP server
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.server.Shutdown(ctx); err != nil {
			h.logger.WithError(err).Error("Error shutting down HTTP server")
			return err
		}
	}

	close(h.messagesCh)
	h.logger.Info("HTTP SSE transport closed")
	return nil
}

// IsClosed returns whether the transport is closed
func (h *HTTPSSETransport) IsClosed() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.closed
}

// GetType returns the transport type
func (h *HTTPSSETransport) GetType() string {
	return "http-sse"
}

// GetConnectedClients returns the number of connected clients
func (h *HTTPSSETransport) GetConnectedClients() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}