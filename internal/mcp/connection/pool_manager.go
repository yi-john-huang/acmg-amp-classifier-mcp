package connection

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// ConnectionPoolConfig defines configuration for connection pool management
type ConnectionPoolConfig struct {
	// Maximum number of connections per pool
	MaxConnections int
	// Minimum idle connections to maintain
	MinIdleConnections int
	// Maximum idle time before connection cleanup
	MaxIdleTime time.Duration
	// Connection timeout for new connections
	ConnectTimeout time.Duration
	// Health check interval
	HealthCheckInterval time.Duration
	// Enable connection reuse
	EnableReuse bool
	// Enable connection monitoring
	EnableMonitoring bool
	// Pool-specific configurations
	PoolConfigs map[string]PoolConfig
}

// PoolConfig defines configuration for a specific connection pool
type PoolConfig struct {
	MaxConn     int           `json:"max_connections"`
	MinIdle     int           `json:"min_idle"`
	IdleTimeout time.Duration `json:"idle_timeout"`
	DialTimeout time.Duration `json:"dial_timeout"`
	Protocol    string        `json:"protocol"` // "tcp", "unix", "websocket"
	Address     string        `json:"address"`
	Enabled     bool          `json:"enabled"`
}

// Connection represents a managed connection in the pool
type Connection struct {
	ID            string              `json:"id"`
	Conn          net.Conn            `json:"-"`
	Protocol      string              `json:"protocol"`
	Address       string              `json:"address"`
	CreatedAt     time.Time           `json:"created_at"`
	LastUsed      time.Time           `json:"last_used"`
	UseCount      int64               `json:"use_count"`
	IsIdle        bool                `json:"is_idle"`
	ClientID      string              `json:"client_id"`
	Metadata      map[string]string   `json:"metadata"`
	HealthStatus  ConnectionHealth    `json:"health_status"`
}

// ConnectionHealth tracks the health status of a connection
type ConnectionHealth struct {
	IsHealthy     bool      `json:"is_healthy"`
	LastCheck     time.Time `json:"last_check"`
	FailureCount  int       `json:"failure_count"`
	LatencyMs     int64     `json:"latency_ms"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// ConnectionPool manages a pool of connections for a specific protocol/address
type ConnectionPool struct {
	name        string
	config      PoolConfig
	connections map[string]*Connection
	idleConns   []*Connection
	mutex       sync.RWMutex
	stats       PoolStats
	statsMutex  sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// PoolStats tracks connection pool performance metrics
type PoolStats struct {
	ActiveConnections   int64         `json:"active_connections"`
	IdleConnections     int64         `json:"idle_connections"`
	TotalConnections    int64         `json:"total_connections"`
	ConnectionsCreated  int64         `json:"connections_created"`
	ConnectionsClosed   int64         `json:"connections_closed"`
	ConnectionsReused   int64         `json:"connections_reused"`
	FailedConnections   int64         `json:"failed_connections"`
	AverageLatency      time.Duration `json:"average_latency"`
	PeakConnections     int64         `json:"peak_connections"`
	HealthChecksFailed  int64         `json:"health_checks_failed"`
	PoolUtilization     float64       `json:"pool_utilization"`
}

// PoolManager manages multiple connection pools for different protocols and endpoints
type PoolManager struct {
	config    ConnectionPoolConfig
	pools     map[string]*ConnectionPool
	poolMutex sync.RWMutex
	stats     ManagerStats
	statsMutex sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// ManagerStats tracks overall pool manager statistics
type ManagerStats struct {
	TotalPools          int                  `json:"total_pools"`
	TotalConnections    int64                `json:"total_connections"`
	ActivePools         int                  `json:"active_pools"`
	PoolStats           map[string]PoolStats `json:"pool_stats"`
	GlobalUtilization   float64              `json:"global_utilization"`
	HealthCheckInterval time.Duration        `json:"health_check_interval"`
}

// NewPoolManager creates a new connection pool manager
func NewPoolManager(config ConnectionPoolConfig) *PoolManager {
	if config.MaxConnections == 0 {
		config.MaxConnections = 100
	}
	if config.MinIdleConnections == 0 {
		config.MinIdleConnections = 5
	}
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = 30 * time.Minute
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.PoolConfigs == nil {
		config.PoolConfigs = make(map[string]PoolConfig)
	}

	pm := &PoolManager{
		config: config,
		pools:  make(map[string]*ConnectionPool),
		stats: ManagerStats{
			PoolStats: make(map[string]PoolStats),
		},
		stopCh: make(chan struct{}),
	}

	// Start background maintenance
	pm.startBackgroundTasks()

	return pm
}

// GetConnection retrieves or creates a connection from the appropriate pool
func (pm *PoolManager) GetConnection(ctx context.Context, poolName string, clientID string) (*Connection, error) {
	pool, err := pm.getOrCreatePool(poolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool: %w", err)
	}

	return pool.GetConnection(ctx, clientID)
}

// ReturnConnection returns a connection to its pool
func (pm *PoolManager) ReturnConnection(poolName string, conn *Connection) error {
	pool := pm.getPool(poolName)
	if pool == nil {
		return fmt.Errorf("pool not found: %s", poolName)
	}

	return pool.ReturnConnection(conn)
}

// CreatePool creates a new connection pool with the given configuration
func (pm *PoolManager) CreatePool(name string, config PoolConfig) error {
	pm.poolMutex.Lock()
	defer pm.poolMutex.Unlock()

	if _, exists := pm.pools[name]; exists {
		return fmt.Errorf("pool already exists: %s", name)
	}

	pool := NewConnectionPool(name, config)
	pm.pools[name] = pool

	pm.statsMutex.Lock()
	pm.stats.TotalPools++
	if config.Enabled {
		pm.stats.ActivePools++
	}
	pm.statsMutex.Unlock()

	return nil
}

// RemovePool removes a connection pool and closes all its connections
func (pm *PoolManager) RemovePool(name string) error {
	pm.poolMutex.Lock()
	defer pm.poolMutex.Unlock()

	pool, exists := pm.pools[name]
	if !exists {
		return fmt.Errorf("pool not found: %s", name)
	}

	pool.Close()
	delete(pm.pools, name)

	pm.statsMutex.Lock()
	pm.stats.TotalPools--
	pm.stats.ActivePools--
	delete(pm.stats.PoolStats, name)
	pm.statsMutex.Unlock()

	return nil
}

// GetPoolStats returns statistics for a specific pool
func (pm *PoolManager) GetPoolStats(poolName string) (PoolStats, bool) {
	pool := pm.getPool(poolName)
	if pool == nil {
		return PoolStats{}, false
	}

	return pool.GetStats(), true
}

// GetManagerStats returns overall manager statistics
func (pm *PoolManager) GetManagerStats() ManagerStats {
	pm.statsMutex.Lock()
	defer pm.statsMutex.Unlock()

	// Update pool stats
	pm.poolMutex.RLock()
	for name, pool := range pm.pools {
		pm.stats.PoolStats[name] = pool.GetStats()
	}
	pm.poolMutex.RUnlock()

	// Calculate global utilization
	var totalConnections, maxConnections int64
	for _, stats := range pm.stats.PoolStats {
		totalConnections += stats.TotalConnections
		maxConnections += int64(pm.config.MaxConnections)
	}

	if maxConnections > 0 {
		pm.stats.GlobalUtilization = float64(totalConnections) / float64(maxConnections)
	}

	pm.stats.TotalConnections = totalConnections
	pm.stats.HealthCheckInterval = pm.config.HealthCheckInterval

	return pm.stats
}

// Close closes the pool manager and all managed pools
func (pm *PoolManager) Close() error {
	close(pm.stopCh)
	pm.wg.Wait()

	pm.poolMutex.Lock()
	defer pm.poolMutex.Unlock()

	var errors []string
	for name, pool := range pm.pools {
		if err := pool.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("pool %s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing pools: %v", errors)
	}

	return nil
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(name string, config PoolConfig) *ConnectionPool {
	if config.MaxConn == 0 {
		config.MaxConn = 50
	}
	if config.MinIdle == 0 {
		config.MinIdle = 2
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 15 * time.Minute
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}

	pool := &ConnectionPool{
		name:        name,
		config:      config,
		connections: make(map[string]*Connection),
		idleConns:   make([]*Connection, 0, config.MaxConn),
		stopCh:      make(chan struct{}),
	}

	// Start background maintenance for this pool
	pool.startMaintenance()

	return pool
}

// GetConnection gets a connection from the pool
func (cp *ConnectionPool) GetConnection(ctx context.Context, clientID string) (*Connection, error) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Try to reuse an idle connection
	if len(cp.idleConns) > 0 {
		conn := cp.idleConns[len(cp.idleConns)-1]
		cp.idleConns = cp.idleConns[:len(cp.idleConns)-1]
		
		conn.IsIdle = false
		conn.LastUsed = time.Now()
		conn.UseCount++
		conn.ClientID = clientID

		cp.updateStats(false, true, false, false)
		return conn, nil
	}

	// Check if we can create a new connection
	if len(cp.connections) >= cp.config.MaxConn {
		return nil, fmt.Errorf("connection pool exhausted for %s", cp.name)
	}

	// Create new connection
	conn, err := cp.createConnection(ctx, clientID)
	if err != nil {
		cp.updateStats(false, false, true, false)
		return nil, err
	}

	cp.connections[conn.ID] = conn
	cp.updateStats(true, false, false, false)
	return conn, nil
}

// ReturnConnection returns a connection to the pool
func (cp *ConnectionPool) ReturnConnection(conn *Connection) error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	if conn == nil {
		return fmt.Errorf("cannot return nil connection")
	}

	// Check if connection is still healthy
	if !cp.isConnectionHealthy(conn) {
		cp.closeConnection(conn)
		delete(cp.connections, conn.ID)
		return nil
	}

	// Add to idle pool if there's room
	if len(cp.idleConns) < cp.config.MaxConn {
		conn.IsIdle = true
		conn.ClientID = ""
		cp.idleConns = append(cp.idleConns, conn)
	} else {
		// Close excess connections
		cp.closeConnection(conn)
		delete(cp.connections, conn.ID)
	}

	return nil
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() PoolStats {
	cp.statsMutex.RLock()
	defer cp.statsMutex.RUnlock()

	cp.mutex.RLock()
	activeConnections := int64(len(cp.connections) - len(cp.idleConns))
	idleConnections := int64(len(cp.idleConns))
	totalConnections := int64(len(cp.connections))
	cp.mutex.RUnlock()

	stats := cp.stats
	stats.ActiveConnections = activeConnections
	stats.IdleConnections = idleConnections
	stats.TotalConnections = totalConnections

	// Calculate utilization
	if cp.config.MaxConn > 0 {
		stats.PoolUtilization = float64(totalConnections) / float64(cp.config.MaxConn)
	}

	return stats
}

// Close closes the connection pool
func (cp *ConnectionPool) Close() error {
	close(cp.stopCh)
	cp.wg.Wait()

	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	var errors []string
	for _, conn := range cp.connections {
		if err := cp.closeConnection(conn); err != nil {
			errors = append(errors, err.Error())
		}
	}

	cp.connections = make(map[string]*Connection)
	cp.idleConns = make([]*Connection, 0)

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}

	return nil
}

// Private helper methods for PoolManager

func (pm *PoolManager) getOrCreatePool(poolName string) (*ConnectionPool, error) {
	pool := pm.getPool(poolName)
	if pool != nil {
		return pool, nil
	}

	// Create pool with default or configured settings
	config, exists := pm.config.PoolConfigs[poolName]
	if !exists {
		// Create default config
		config = PoolConfig{
			MaxConn:     pm.config.MaxConnections,
			MinIdle:     pm.config.MinIdleConnections,
			IdleTimeout: pm.config.MaxIdleTime,
			DialTimeout: pm.config.ConnectTimeout,
			Protocol:    "tcp",
			Enabled:     true,
		}
	}

	return pm.createPoolWithConfig(poolName, config)
}

func (pm *PoolManager) getPool(poolName string) *ConnectionPool {
	pm.poolMutex.RLock()
	defer pm.poolMutex.RUnlock()
	return pm.pools[poolName]
}

func (pm *PoolManager) createPoolWithConfig(name string, config PoolConfig) (*ConnectionPool, error) {
	pm.poolMutex.Lock()
	defer pm.poolMutex.Unlock()

	if _, exists := pm.pools[name]; exists {
		return pm.pools[name], nil
	}

	pool := NewConnectionPool(name, config)
	pm.pools[name] = pool

	return pool, nil
}

func (pm *PoolManager) startBackgroundTasks() {
	if pm.config.EnableMonitoring {
		pm.wg.Add(1)
		go pm.monitorPools()
	}
}

func (pm *PoolManager) monitorPools() {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.performHealthChecks()
		case <-pm.stopCh:
			return
		}
	}
}

func (pm *PoolManager) performHealthChecks() {
	pm.poolMutex.RLock()
	pools := make([]*ConnectionPool, 0, len(pm.pools))
	for _, pool := range pm.pools {
		pools = append(pools, pool)
	}
	pm.poolMutex.RUnlock()

	for _, pool := range pools {
		pool.performHealthCheck()
	}
}

// Private helper methods for ConnectionPool

func (cp *ConnectionPool) createConnection(ctx context.Context, clientID string) (*Connection, error) {
	var netConn net.Conn
	var err error

	// Set dial timeout
	dialCtx, cancel := context.WithTimeout(ctx, cp.config.DialTimeout)
	defer cancel()

	// Create connection based on protocol
	switch cp.config.Protocol {
	case "tcp":
		dialer := &net.Dialer{}
		netConn, err = dialer.DialContext(dialCtx, "tcp", cp.config.Address)
	case "unix":
		dialer := &net.Dialer{}
		netConn, err = dialer.DialContext(dialCtx, "unix", cp.config.Address)
	default:
		err = fmt.Errorf("unsupported protocol: %s", cp.config.Protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	conn := &Connection{
		ID:        cp.generateConnectionID(),
		Conn:      netConn,
		Protocol:  cp.config.Protocol,
		Address:   cp.config.Address,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		UseCount:  1,
		IsIdle:    false,
		ClientID:  clientID,
		Metadata:  make(map[string]string),
		HealthStatus: ConnectionHealth{
			IsHealthy: true,
			LastCheck: time.Now(),
		},
	}

	return conn, nil
}

func (cp *ConnectionPool) generateConnectionID() string {
	return fmt.Sprintf("%s-%d", cp.name, time.Now().UnixNano())
}

func (cp *ConnectionPool) isConnectionHealthy(conn *Connection) bool {
	// Simple health check - could be enhanced
	if conn.Conn == nil {
		return false
	}

	// Check if connection is stale
	if time.Since(conn.LastUsed) > cp.config.IdleTimeout {
		return false
	}

	return true
}

func (cp *ConnectionPool) closeConnection(conn *Connection) error {
	if conn.Conn != nil {
		return conn.Conn.Close()
	}
	return nil
}

func (cp *ConnectionPool) updateStats(created, reused, failed, closed bool) {
	cp.statsMutex.Lock()
	defer cp.statsMutex.Unlock()

	if created {
		cp.stats.ConnectionsCreated++
		if cp.stats.TotalConnections > cp.stats.PeakConnections {
			cp.stats.PeakConnections = cp.stats.TotalConnections
		}
	}
	if reused {
		cp.stats.ConnectionsReused++
	}
	if failed {
		cp.stats.FailedConnections++
	}
	if closed {
		cp.stats.ConnectionsClosed++
	}
}

func (cp *ConnectionPool) startMaintenance() {
	cp.wg.Add(1)
	go cp.maintenanceLoop()
}

func (cp *ConnectionPool) maintenanceLoop() {
	defer cp.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.cleanupIdleConnections()
		case <-cp.stopCh:
			return
		}
	}
}

func (cp *ConnectionPool) cleanupIdleConnections() {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	now := time.Now()
	newIdleConns := make([]*Connection, 0, len(cp.idleConns))

	for _, conn := range cp.idleConns {
		if now.Sub(conn.LastUsed) > cp.config.IdleTimeout {
			// Connection is too old, close it
			cp.closeConnection(conn)
			delete(cp.connections, conn.ID)
			cp.updateStats(false, false, false, true)
		} else {
			newIdleConns = append(newIdleConns, conn)
		}
	}

	cp.idleConns = newIdleConns
}

func (cp *ConnectionPool) performHealthCheck() {
	cp.mutex.RLock()
	connections := make([]*Connection, 0, len(cp.connections))
	for _, conn := range cp.connections {
		connections = append(connections, conn)
	}
	cp.mutex.RUnlock()

	for _, conn := range connections {
		if !cp.isConnectionHealthy(conn) {
			cp.mutex.Lock()
			cp.closeConnection(conn)
			delete(cp.connections, conn.ID)
			// Remove from idle connections if present
			for i, idleConn := range cp.idleConns {
				if idleConn.ID == conn.ID {
					cp.idleConns = append(cp.idleConns[:i], cp.idleConns[i+1:]...)
					break
				}
			}
			cp.mutex.Unlock()
			cp.updateStats(false, false, false, true)
			cp.statsMutex.Lock()
			cp.stats.HealthChecksFailed++
			cp.statsMutex.Unlock()
		}
	}
}