package connection

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock connection that implements net.Conn
type mockConn struct {
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9090}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestNewPoolManager(t *testing.T) {
	config := ConnectionPoolConfig{}
	manager := NewPoolManager(config)

	assert.NotNil(t, manager)
	assert.Equal(t, 100, manager.config.MaxConnections)
	assert.Equal(t, 5, manager.config.MinIdleConnections)
	assert.Equal(t, 30*time.Minute, manager.config.MaxIdleTime)

	// Cleanup
	manager.Close()
}

func TestNewConnectionPool(t *testing.T) {
	config := PoolConfig{
		MaxConn:     10,
		MinIdle:     2,
		IdleTimeout: 5 * time.Minute,
		Protocol:    "tcp",
		Address:     "localhost:8080",
		Enabled:     true,
	}

	pool := NewConnectionPool("test-pool", config)

	assert.NotNil(t, pool)
	assert.Equal(t, "test-pool", pool.name)
	assert.Equal(t, config.MaxConn, pool.config.MaxConn)
	assert.Equal(t, config.MinIdle, pool.config.MinIdle)

	// Cleanup
	pool.Close()
}

func TestCreatePool(t *testing.T) {
	manager := NewPoolManager(ConnectionPoolConfig{})
	defer manager.Close()

	config := PoolConfig{
		MaxConn: 5,
		MinIdle: 1,
		Enabled: true,
	}

	err := manager.CreatePool("test-pool", config)
	require.NoError(t, err)

	// Verify pool exists
	pool := manager.getPool("test-pool")
	assert.NotNil(t, pool)
	assert.Equal(t, "test-pool", pool.name)

	// Try to create duplicate pool
	err = manager.CreatePool("test-pool", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pool already exists")
}

func TestRemovePool(t *testing.T) {
	manager := NewPoolManager(ConnectionPoolConfig{})
	defer manager.Close()

	config := PoolConfig{
		MaxConn: 5,
		Enabled: true,
	}

	// Create pool
	err := manager.CreatePool("test-pool", config)
	require.NoError(t, err)

	// Verify it exists
	pool := manager.getPool("test-pool")
	assert.NotNil(t, pool)

	// Remove pool
	err = manager.RemovePool("test-pool")
	require.NoError(t, err)

	// Verify it's gone
	pool = manager.getPool("test-pool")
	assert.Nil(t, pool)

	// Try to remove non-existent pool
	err = manager.RemovePool("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pool not found")
}

func TestConnectionPoolStats(t *testing.T) {
	config := PoolConfig{
		MaxConn:     10,
		MinIdle:     2,
		IdleTimeout: 5 * time.Minute,
		Protocol:    "tcp",
		Enabled:     true,
	}

	pool := NewConnectionPool("test-pool", config)
	defer pool.Close()

	stats := pool.GetStats()
	assert.Equal(t, int64(0), stats.ActiveConnections)
	assert.Equal(t, int64(0), stats.IdleConnections)
	assert.Equal(t, int64(0), stats.TotalConnections)
}

func TestManagerStats(t *testing.T) {
	manager := NewPoolManager(ConnectionPoolConfig{
		EnableMonitoring: true,
	})
	defer manager.Close()

	// Create a couple of pools
	manager.CreatePool("pool1", PoolConfig{MaxConn: 5, Enabled: true})
	manager.CreatePool("pool2", PoolConfig{MaxConn: 10, Enabled: true})

	stats := manager.GetManagerStats()
	assert.Equal(t, 2, stats.TotalPools)
	assert.Equal(t, 2, stats.ActivePools)
	assert.Contains(t, stats.PoolStats, "pool1")
	assert.Contains(t, stats.PoolStats, "pool2")
}

func TestConnectionCreation(t *testing.T) {
	config := PoolConfig{
		MaxConn:     2,
		MinIdle:     1,
		IdleTimeout: 5 * time.Minute,
		DialTimeout: 1 * time.Second,
		Protocol:    "tcp",
		Address:     "127.0.0.1:0", // Use port 0 for testing
		Enabled:     true,
	}

	pool := NewConnectionPool("test-pool", config)
	defer pool.Close()

	// Mock the connection creation by replacing the createConnection method
	// In a real test, you would set up a test server
	
	// Test that we respect max connections
	pool.mutex.Lock()
	pool.config.MaxConn = 1
	
	// Manually add a connection to simulate pool exhaustion
	mockConn := &Connection{
		ID:       "test-conn",
		Conn:     &mockConn{},
		IsIdle:   false,
		LastUsed: time.Now(),
	}
	pool.connections["test-conn"] = mockConn
	pool.mutex.Unlock()

	// This should fail due to pool exhaustion
	_, err := pool.GetConnection(context.Background(), "client1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection pool exhausted")
}

func TestReturnConnection(t *testing.T) {
	config := PoolConfig{
		MaxConn:     5,
		MinIdle:     1,
		IdleTimeout: 5 * time.Minute,
		Protocol:    "tcp",
		Enabled:     true,
	}

	pool := NewConnectionPool("test-pool", config)
	defer pool.Close()

	// Create a mock connection
	conn := &Connection{
		ID:       "test-conn",
		Conn:     &mockConn{},
		IsIdle:   false,
		LastUsed: time.Now(),
		ClientID: "client1",
	}

	// Add to active connections
	pool.mutex.Lock()
	pool.connections[conn.ID] = conn
	pool.mutex.Unlock()

	// Return the connection
	err := pool.ReturnConnection(conn)
	require.NoError(t, err)

	// Check that connection is now idle
	pool.mutex.RLock()
	assert.True(t, conn.IsIdle)
	assert.Empty(t, conn.ClientID)
	assert.Len(t, pool.idleConns, 1)
	pool.mutex.RUnlock()
}

func TestConnectionReuse(t *testing.T) {
	config := PoolConfig{
		MaxConn:     5,
		MinIdle:     1,
		IdleTimeout: 5 * time.Minute,
		Protocol:    "tcp",
		Enabled:     true,
	}

	pool := NewConnectionPool("test-pool", config)
	defer pool.Close()

	// Manually add an idle connection
	idleConn := &Connection{
		ID:       "idle-conn",
		Conn:     &mockConn{},
		IsIdle:   true,
		LastUsed: time.Now(),
		UseCount: 5,
	}

	pool.mutex.Lock()
	pool.connections[idleConn.ID] = idleConn
	pool.idleConns = append(pool.idleConns, idleConn)
	pool.mutex.Unlock()

	// Get connection should reuse the idle one
	conn, err := pool.GetConnection(context.Background(), "client1")
	require.NoError(t, err)

	assert.Equal(t, "idle-conn", conn.ID)
	assert.Equal(t, "client1", conn.ClientID)
	assert.False(t, conn.IsIdle)
	assert.Equal(t, int64(6), conn.UseCount) // Should increment

	// Verify idle pool is now empty
	pool.mutex.RLock()
	assert.Len(t, pool.idleConns, 0)
	pool.mutex.RUnlock()
}

func TestConnectionHealthCheck(t *testing.T) {
	config := PoolConfig{
		MaxConn:     5,
		MinIdle:     1,
		IdleTimeout: 100 * time.Millisecond, // Very short for testing
		Protocol:    "tcp",
		Enabled:     true,
	}

	pool := NewConnectionPool("test-pool", config)
	defer pool.Close()

	// Add a connection that will become stale
	staleConn := &Connection{
		ID:       "stale-conn",
		Conn:     &mockConn{},
		IsIdle:   true,
		LastUsed: time.Now().Add(-200 * time.Millisecond), // Old timestamp
	}

	pool.mutex.Lock()
	pool.connections[staleConn.ID] = staleConn
	pool.idleConns = append(pool.idleConns, staleConn)
	pool.mutex.Unlock()

	// Check that connection is considered unhealthy
	healthy := pool.isConnectionHealthy(staleConn)
	assert.False(t, healthy)

	// Cleanup should remove stale connections
	pool.cleanupIdleConnections()

	pool.mutex.RLock()
	assert.Len(t, pool.connections, 0)
	assert.Len(t, pool.idleConns, 0)
	pool.mutex.RUnlock()
}

func TestPoolManagerAutoCreate(t *testing.T) {
	manager := NewPoolManager(ConnectionPoolConfig{
		MaxConnections: 10,
		PoolConfigs: map[string]PoolConfig{
			"configured-pool": {
				MaxConn: 15,
				Enabled: true,
			},
		},
	})
	defer manager.Close()

	// Try to get connection from non-existent pool (should auto-create with defaults)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := manager.GetConnection(ctx, "auto-created-pool", "client1")
	// This will fail because we can't actually connect, but the pool should be created
	assert.Error(t, err) // Connection will fail, but that's expected

	// Verify pool was created
	pool := manager.getPool("auto-created-pool")
	assert.NotNil(t, pool)

	// Try with configured pool
	_, err = manager.GetConnection(ctx, "configured-pool", "client2")
	assert.Error(t, err) // Connection will fail, but that's expected

	pool = manager.getPool("configured-pool")
	assert.NotNil(t, pool)
	assert.Equal(t, 15, pool.config.MaxConn) // Should use configured value
}

func TestReturnNilConnection(t *testing.T) {
	pool := NewConnectionPool("test-pool", PoolConfig{MaxConn: 5, Enabled: true})
	defer pool.Close()

	err := pool.ReturnConnection(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot return nil connection")
}

func TestClosePoolWithConnections(t *testing.T) {
	pool := NewConnectionPool("test-pool", PoolConfig{MaxConn: 5, Enabled: true})

	// Add some mock connections
	for i := 0; i < 3; i++ {
		conn := &Connection{
			ID:   fmt.Sprintf("conn-%d", i),
			Conn: &mockConn{},
		}
		pool.connections[conn.ID] = conn
	}

	err := pool.Close()
	assert.NoError(t, err)

	// Verify all connections are closed
	assert.Len(t, pool.connections, 0)
	assert.Len(t, pool.idleConns, 0)
}

func TestPoolUtilization(t *testing.T) {
	config := PoolConfig{
		MaxConn: 10,
		Enabled: true,
	}

	pool := NewConnectionPool("test-pool", config)
	defer pool.Close()

	// Add some connections
	for i := 0; i < 5; i++ {
		conn := &Connection{
			ID:     fmt.Sprintf("conn-%d", i),
			Conn:   &mockConn{},
			IsIdle: i < 2, // First 2 are idle
		}
		pool.connections[conn.ID] = conn
		if conn.IsIdle {
			pool.idleConns = append(pool.idleConns, conn)
		}
	}

	stats := pool.GetStats()
	assert.Equal(t, int64(3), stats.ActiveConnections) // 5 total - 2 idle
	assert.Equal(t, int64(2), stats.IdleConnections)
	assert.Equal(t, int64(5), stats.TotalConnections)
	assert.Equal(t, 0.5, stats.PoolUtilization) // 5/10
}

func TestManagerWithMonitoring(t *testing.T) {
	manager := NewPoolManager(ConnectionPoolConfig{
		EnableMonitoring:    true,
		HealthCheckInterval: 50 * time.Millisecond,
	})

	// Give some time for monitoring to start
	time.Sleep(100 * time.Millisecond)

	manager.Close()
}

func TestGenerateConnectionID(t *testing.T) {
	pool := NewConnectionPool("test-pool", PoolConfig{})
	defer pool.Close()

	id1 := pool.generateConnectionID()
	id2 := pool.generateConnectionID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "test-pool")
	assert.Contains(t, id2, "test-pool")
}