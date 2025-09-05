package protocol

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RateLimiter implements rate limiting for MCP clients
type RateLimiter struct {
	logger      *logrus.Logger
	clients     map[string]*ClientLimiter
	config      *RateLimitConfig
	mu          sync.RWMutex
}

// ClientLimiter tracks rate limiting for a specific client
type ClientLimiter struct {
	ClientID        string        `json:"client_id"`
	RequestCount    int64         `json:"request_count"`
	LastReset       time.Time     `json:"last_reset"`
	WindowDuration  time.Duration `json:"window_duration"`
	RequestLimit    int64         `json:"request_limit"`
	BurstLimit      int64         `json:"burst_limit"`
	TokenBucket     int64         `json:"current_tokens"`
	LastRefill      time.Time     `json:"last_refill"`
	ViolationCount  int64         `json:"violation_count"`
	LastViolation   *time.Time    `json:"last_violation,omitempty"`
	Blocked         bool          `json:"blocked"`
	BlockedUntil    *time.Time    `json:"blocked_until,omitempty"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enabled            bool          `json:"enabled"`
	RequestsPerMinute  int64         `json:"requests_per_minute"`
	RequestsPerHour    int64         `json:"requests_per_hour"`
	BurstLimit         int64         `json:"burst_limit"`
	WindowDuration     time.Duration `json:"window_duration"`
	BlockDuration      time.Duration `json:"block_duration"`
	MaxViolations      int64         `json:"max_violations"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(logger *logrus.Logger) *RateLimiter {
	rl := &RateLimiter{
		logger:  logger,
		clients: make(map[string]*ClientLimiter),
		config: &RateLimitConfig{
			Enabled:            true,
			RequestsPerMinute:  60,   // 60 requests per minute (1 per second average)
			RequestsPerHour:    1000, // 1000 requests per hour
			BurstLimit:         10,   // Allow short bursts up to 10 requests
			WindowDuration:     time.Minute,
			BlockDuration:      5 * time.Minute,
			MaxViolations:      5,
			CleanupInterval:    10 * time.Minute,
		},
	}

	// Start cleanup goroutine
	go rl.startCleanupRoutine()

	return rl
}

// InitializeClient initializes rate limiting for a new client
func (rl *RateLimiter) InitializeClient(clientID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rl.clients[clientID] = &ClientLimiter{
		ClientID:        clientID,
		RequestCount:    0,
		LastReset:       now,
		WindowDuration:  rl.config.WindowDuration,
		RequestLimit:    rl.config.RequestsPerMinute,
		BurstLimit:      rl.config.BurstLimit,
		TokenBucket:     rl.config.BurstLimit, // Start with full token bucket
		LastRefill:      now,
		ViolationCount:  0,
		Blocked:         false,
	}

	rl.logger.WithFields(logrus.Fields{
		"client_id":          clientID,
		"requests_per_minute": rl.config.RequestsPerMinute,
		"burst_limit":        rl.config.BurstLimit,
	}).Debug("Initialized rate limiter for client")
}

// AllowRequest checks if a request should be allowed for the given client
func (rl *RateLimiter) AllowRequest(clientID string) bool {
	if !rl.config.Enabled {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		// Auto-initialize client if not exists
		rl.initializeClientUnsafe(clientID)
		client = rl.clients[clientID]
	}

	now := time.Now()

	// Check if client is currently blocked
	if client.Blocked && client.BlockedUntil != nil && now.Before(*client.BlockedUntil) {
		rl.logger.WithFields(logrus.Fields{
			"client_id":    clientID,
			"blocked_until": client.BlockedUntil.Format(time.RFC3339),
		}).Debug("Request denied: client is blocked")
		return false
	}

	// Unblock client if block period has expired
	if client.Blocked && client.BlockedUntil != nil && !now.Before(*client.BlockedUntil) {
		client.Blocked = false
		client.BlockedUntil = nil
		rl.logger.WithField("client_id", clientID).Info("Client unblocked after timeout")
	}

	// Refill token bucket based on time elapsed
	rl.refillTokenBucket(client, now)

	// Check token bucket for burst protection
	if client.TokenBucket <= 0 {
		rl.recordViolation(client, now)
		rl.logger.WithFields(logrus.Fields{
			"client_id":       clientID,
			"violation_count": client.ViolationCount,
		}).Warn("Request denied: token bucket empty (burst limit exceeded)")
		return false
	}

	// Check sliding window rate limit
	rl.updateSlidingWindow(client, now)
	if client.RequestCount >= client.RequestLimit {
		rl.recordViolation(client, now)
		rl.logger.WithFields(logrus.Fields{
			"client_id":     clientID,
			"request_count": client.RequestCount,
			"limit":         client.RequestLimit,
			"violation_count": client.ViolationCount,
		}).Warn("Request denied: rate limit exceeded")
		return false
	}

	// Allow request and consume token
	client.RequestCount++
	client.TokenBucket--

	rl.logger.WithFields(logrus.Fields{
		"client_id":       clientID,
		"request_count":   client.RequestCount,
		"tokens_remaining": client.TokenBucket,
	}).Debug("Request allowed")

	return true
}

// initializeClientUnsafe initializes a client without locking (internal use)
func (rl *RateLimiter) initializeClientUnsafe(clientID string) {
	now := time.Now()
	rl.clients[clientID] = &ClientLimiter{
		ClientID:        clientID,
		RequestCount:    0,
		LastReset:       now,
		WindowDuration:  rl.config.WindowDuration,
		RequestLimit:    rl.config.RequestsPerMinute,
		BurstLimit:      rl.config.BurstLimit,
		TokenBucket:     rl.config.BurstLimit,
		LastRefill:      now,
		ViolationCount:  0,
		Blocked:         false,
	}
}

// refillTokenBucket refills the token bucket based on elapsed time
func (rl *RateLimiter) refillTokenBucket(client *ClientLimiter, now time.Time) {
	elapsed := now.Sub(client.LastRefill)
	
	// Refill rate: burst limit tokens per window duration
	tokensToAdd := int64(elapsed.Seconds() * float64(client.BurstLimit) / client.WindowDuration.Seconds())
	
	if tokensToAdd > 0 {
		client.TokenBucket += tokensToAdd
		if client.TokenBucket > client.BurstLimit {
			client.TokenBucket = client.BurstLimit
		}
		client.LastRefill = now
	}
}

// updateSlidingWindow updates the sliding window request count
func (rl *RateLimiter) updateSlidingWindow(client *ClientLimiter, now time.Time) {
	// Reset window if enough time has passed
	if now.Sub(client.LastReset) >= client.WindowDuration {
		client.RequestCount = 0
		client.LastReset = now
	}
}

// recordViolation records a rate limit violation
func (rl *RateLimiter) recordViolation(client *ClientLimiter, now time.Time) {
	client.ViolationCount++
	client.LastViolation = &now

	// Block client if too many violations
	if client.ViolationCount >= rl.config.MaxViolations {
		client.Blocked = true
		blockedUntil := now.Add(rl.config.BlockDuration)
		client.BlockedUntil = &blockedUntil

		rl.logger.WithFields(logrus.Fields{
			"client_id":       client.ClientID,
			"violation_count": client.ViolationCount,
			"blocked_until":   blockedUntil.Format(time.RFC3339),
		}).Warn("Client blocked due to excessive rate limit violations")
	}
}

// RemoveClient removes rate limiting data for a client
func (rl *RateLimiter) RemoveClient(clientID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.clients, clientID)
	rl.logger.WithField("client_id", clientID).Debug("Removed rate limiter data for client")
}

// GetClientStats returns rate limiting statistics for a specific client
func (rl *RateLimiter) GetClientStats(clientID string) map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	client, exists := rl.clients[clientID]
	if !exists {
		return nil
	}

	stats := map[string]interface{}{
		"client_id":       client.ClientID,
		"request_count":   client.RequestCount,
		"tokens_remaining": client.TokenBucket,
		"violation_count": client.ViolationCount,
		"blocked":         client.Blocked,
		"window_reset_in": client.WindowDuration - time.Since(client.LastReset),
	}

	if client.BlockedUntil != nil {
		stats["blocked_until"] = client.BlockedUntil.Format(time.RFC3339)
		stats["unblock_in"] = time.Until(*client.BlockedUntil)
	}

	return stats
}

// GetStats returns overall rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	totalClients := len(rl.clients)
	blockedClients := 0
	totalViolations := int64(0)
	totalRequests := int64(0)

	for _, client := range rl.clients {
		if client.Blocked {
			blockedClients++
		}
		totalViolations += client.ViolationCount
		totalRequests += client.RequestCount
	}

	return map[string]interface{}{
		"enabled":               rl.config.Enabled,
		"total_clients":         totalClients,
		"blocked_clients":       blockedClients,
		"total_violations":      totalViolations,
		"total_requests":        totalRequests,
		"requests_per_minute":   rl.config.RequestsPerMinute,
		"burst_limit":           rl.config.BurstLimit,
		"block_duration":        rl.config.BlockDuration.String(),
		"max_violations":        rl.config.MaxViolations,
	}
}

// UpdateConfig updates the rate limiting configuration
func (rl *RateLimiter) UpdateConfig(config *RateLimitConfig) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.config = config

	// Update existing clients with new limits
	for _, client := range rl.clients {
		client.RequestLimit = config.RequestsPerMinute
		client.BurstLimit = config.BurstLimit
		client.WindowDuration = config.WindowDuration
		
		// Reset token bucket to new limit if it's higher
		if client.TokenBucket > config.BurstLimit {
			client.TokenBucket = config.BurstLimit
		}
	}

	rl.logger.WithFields(logrus.Fields{
		"enabled":             config.Enabled,
		"requests_per_minute": config.RequestsPerMinute,
		"burst_limit":         config.BurstLimit,
		"max_violations":      config.MaxViolations,
	}).Info("Updated rate limiting configuration")
}

// startCleanupRoutine starts a goroutine that periodically cleans up old client data
func (rl *RateLimiter) startCleanupRoutine() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanupInactiveClients()
	}
}

// cleanupInactiveClients removes data for clients that have been inactive
func (rl *RateLimiter) cleanupInactiveClients() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	inactiveClients := make([]string, 0)

	// Consider clients inactive if they haven't been accessed for 1 hour
	inactiveThreshold := time.Hour

	for clientID, client := range rl.clients {
		lastActivity := client.LastReset
		if client.LastViolation != nil && client.LastViolation.After(lastActivity) {
			lastActivity = *client.LastViolation
		}

		if now.Sub(lastActivity) > inactiveThreshold && !client.Blocked {
			inactiveClients = append(inactiveClients, clientID)
		}
	}

	for _, clientID := range inactiveClients {
		delete(rl.clients, clientID)
	}

	if len(inactiveClients) > 0 {
		rl.logger.WithField("cleaned_count", len(inactiveClients)).Info("Cleaned up inactive rate limiter data")
	}
}