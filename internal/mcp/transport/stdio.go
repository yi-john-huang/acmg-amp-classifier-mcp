package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

// StdioTransport implements MCP communication over stdin/stdout
type StdioTransport struct {
	logger   *logrus.Logger
	reader   *bufio.Scanner
	writer   io.Writer
	mu       sync.RWMutex
	closed   bool
	cancelFn context.CancelFunc
}

// NewStdioTransport creates a new stdio transport for local AI agent connections
func NewStdioTransport(logger *logrus.Logger) *StdioTransport {
	return &StdioTransport{
		logger: logger,
		reader: bufio.NewScanner(os.Stdin),
		writer: os.Stdout,
	}
}

// Start initializes the stdio transport
func (s *StdioTransport) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("transport is closed")
	}

	// Create cancellable context for this transport
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFn = cancel

	s.logger.Info("Starting stdio transport for MCP communication")
	
	// Stdio transport is ready immediately
	return nil
}

// ReadMessage reads a JSON-RPC message from stdin
func (s *StdioTransport) ReadMessage() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, fmt.Errorf("transport is closed")
	}

	if !s.reader.Scan() {
		if err := s.reader.Err(); err != nil {
			s.logger.WithError(err).Error("Failed to read from stdin")
			return nil, fmt.Errorf("failed to read message: %w", err)
		}
		// EOF
		return nil, io.EOF
	}

	message := s.reader.Bytes()
	s.logger.WithField("message_length", len(message)).Debug("Received message via stdio")
	
	return message, nil
}

// WriteMessage writes a JSON-RPC message to stdout
func (s *StdioTransport) WriteMessage(message []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return fmt.Errorf("transport is closed")
	}

	// Write message followed by newline (MCP protocol requirement)
	if _, err := s.writer.Write(message); err != nil {
		s.logger.WithError(err).Error("Failed to write message to stdout")
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	if _, err := s.writer.Write([]byte("\n")); err != nil {
		s.logger.WithError(err).Error("Failed to write newline to stdout")
		return fmt.Errorf("failed to write newline: %w", err)
	}

	s.logger.WithField("message_length", len(message)).Debug("Sent message via stdio")
	return nil
}

// WriteJSONMessage writes a JSON object as a message
func (s *StdioTransport) WriteJSONMessage(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return s.WriteMessage(data)
}

// Close closes the stdio transport
func (s *StdioTransport) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	if s.cancelFn != nil {
		s.cancelFn()
	}
	
	s.logger.Info("Stdio transport closed")
	return nil
}

// IsClosed returns whether the transport is closed
func (s *StdioTransport) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// GetType returns the transport type
func (s *StdioTransport) GetType() string {
	return "stdio"
}