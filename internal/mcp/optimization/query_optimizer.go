package optimization

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// QueryOptimizerConfig defines configuration for database query optimization
type QueryOptimizerConfig struct {
	// Database connection
	DB *sql.DB
	// Enable query caching
	EnableQueryCache bool
	// Query cache TTL
	QueryCacheTTL time.Duration
	// Enable prepared statement caching
	EnablePreparedStatements bool
	// Maximum number of cached prepared statements
	MaxPreparedStatements int
	// Enable query statistics collection
	EnableQueryStats bool
	// Slow query threshold
	SlowQueryThreshold time.Duration
	// Enable connection pooling optimizations
	EnableConnectionPooling bool
	// Maximum idle connections
	MaxIdleConns int
	// Maximum open connections
	MaxOpenConns int
	// Connection max lifetime
	ConnMaxLifetime time.Duration
}

// QueryOptimizer manages database query optimizations for MCP resources and tools
type QueryOptimizer struct {
	config             QueryOptimizerConfig
	queryCache         map[string]*CachedQuery
	queryCacheMutex    sync.RWMutex
	preparedStmts      map[string]*sql.Stmt
	preparedStmtsMutex sync.RWMutex
	queryStats         QueryStats
	queryStatsMutex    sync.RWMutex
}

// CachedQuery represents a cached database query result
type CachedQuery struct {
	Query     string        `json:"query"`
	Args      []interface{} `json:"args"`
	Result    []QueryResult `json:"result"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	HitCount  int64         `json:"hit_count"`
	Duration  time.Duration `json:"duration"`
}

// QueryResult represents a single row result from a database query
type QueryResult struct {
	Columns []string                 `json:"columns"`
	Values  []interface{}            `json:"values"`
	Data    map[string]interface{}   `json:"data"`
}

// QueryStats tracks database query performance metrics
type QueryStats struct {
	TotalQueries      int64                    `json:"total_queries"`
	CachedQueries     int64                    `json:"cached_queries"`
	SlowQueries       int64                    `json:"slow_queries"`
	FailedQueries     int64                    `json:"failed_queries"`
	AverageQueryTime  time.Duration            `json:"average_query_time"`
	QueryDistribution map[string]int64         `json:"query_distribution"`
	PreparedStmtHits  int64                    `json:"prepared_stmt_hits"`
	PreparedStmtMiss  int64                    `json:"prepared_stmt_miss"`
}

// OptimizedQuery represents query execution context with optimization metadata
type OptimizedQuery struct {
	SQL           string
	Args          []interface{}
	UseCache      bool
	UsePrepared   bool
	Timeout       time.Duration
	QueryType     string
	ResourceType  string
}

// NewQueryOptimizer creates a new query optimizer instance
func NewQueryOptimizer(config QueryOptimizerConfig) *QueryOptimizer {
	if config.QueryCacheTTL == 0 {
		config.QueryCacheTTL = 10 * time.Minute
	}
	if config.MaxPreparedStatements == 0 {
		config.MaxPreparedStatements = 100
	}
	if config.SlowQueryThreshold == 0 {
		config.SlowQueryThreshold = time.Second
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 10
	}
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 100
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = time.Hour
	}

	qo := &QueryOptimizer{
		config:        config,
		queryCache:    make(map[string]*CachedQuery),
		preparedStmts: make(map[string]*sql.Stmt),
		queryStats: QueryStats{
			QueryDistribution: make(map[string]int64),
		},
	}

	// Configure connection pooling
	if config.EnableConnectionPooling && config.DB != nil {
		config.DB.SetMaxIdleConns(config.MaxIdleConns)
		config.DB.SetMaxOpenConns(config.MaxOpenConns)
		config.DB.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	return qo
}

// ExecuteQuery executes an optimized database query
func (qo *QueryOptimizer) ExecuteQuery(ctx context.Context, query OptimizedQuery) ([]QueryResult, error) {
	startTime := time.Now()
	queryKey := qo.generateQueryKey(query.SQL, query.Args)
	
	// Check query cache first
	if query.UseCache && qo.config.EnableQueryCache {
		if cached := qo.getCachedQuery(queryKey); cached != nil {
			qo.updateQueryStats(query.QueryType, time.Since(startTime), true, false, false)
			cached.HitCount++
			return cached.Result, nil
		}
	}

	var rows *sql.Rows
	var err error

	// Use prepared statement if enabled
	if query.UsePrepared && qo.config.EnablePreparedStatements {
		stmt, stmtErr := qo.getPreparedStatement(query.SQL)
		if stmtErr == nil {
			rows, err = stmt.QueryContext(ctx, query.Args...)
			qo.updateQueryStats(query.QueryType, time.Since(startTime), false, true, false)
		} else {
			// Fall back to regular query
			rows, err = qo.config.DB.QueryContext(ctx, query.SQL, query.Args...)
			qo.updateQueryStats(query.QueryType, time.Since(startTime), false, false, false)
		}
	} else {
		rows, err = qo.config.DB.QueryContext(ctx, query.SQL, query.Args...)
		qo.updateQueryStats(query.QueryType, time.Since(startTime), false, false, false)
	}

	if err != nil {
		qo.updateQueryStats(query.QueryType, time.Since(startTime), false, false, true)
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	results, err := qo.scanResults(rows)
	if err != nil {
		qo.updateQueryStats(query.QueryType, time.Since(startTime), false, false, true)
		return nil, fmt.Errorf("result scanning failed: %w", err)
	}

	duration := time.Since(startTime)

	// Cache results if enabled
	if query.UseCache && qo.config.EnableQueryCache {
		qo.setCachedQuery(queryKey, &CachedQuery{
			Query:     query.SQL,
			Args:      query.Args,
			Result:    results,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(qo.config.QueryCacheTTL),
			HitCount:  0,
			Duration:  duration,
		})
	}

	qo.updateQueryStats(query.QueryType, duration, false, false, false)
	return results, nil
}

// ExecuteVariantQuery executes optimized queries for variant resources
func (qo *QueryOptimizer) ExecuteVariantQuery(ctx context.Context, variantID string) ([]QueryResult, error) {
	query := OptimizedQuery{
		SQL: `SELECT v.id, v.hgvs_notation, v.chromosome, v.position, v.ref_allele, v.alt_allele,
		             v.gene_symbol, v.transcript_id, v.created_at, v.updated_at
		      FROM variants v 
		      WHERE v.id = $1 OR v.hgvs_notation = $1`,
		Args:         []interface{}{variantID},
		UseCache:     true,
		UsePrepared:  true,
		QueryType:    "variant_lookup",
		ResourceType: "variant",
	}
	
	return qo.ExecuteQuery(ctx, query)
}

// ExecuteInterpretationQuery executes optimized queries for interpretation resources
func (qo *QueryOptimizer) ExecuteInterpretationQuery(ctx context.Context, interpretationID string) ([]QueryResult, error) {
	query := OptimizedQuery{
		SQL: `SELECT i.id, i.variant_id, i.classification, i.confidence_score,
		             i.acmg_criteria, i.evidence_summary, i.clinical_significance,
		             i.created_at, i.updated_at, i.created_by
		      FROM interpretations i
		      WHERE i.id = $1`,
		Args:         []interface{}{interpretationID},
		UseCache:     true,
		UsePrepared:  true,
		QueryType:    "interpretation_lookup",
		ResourceType: "interpretation",
	}
	
	return qo.ExecuteQuery(ctx, query)
}

// ExecuteEvidenceQuery executes optimized queries for evidence resources
func (qo *QueryOptimizer) ExecuteEvidenceQuery(ctx context.Context, variantID string) ([]QueryResult, error) {
	query := OptimizedQuery{
		SQL: `SELECT e.id, e.variant_id, e.evidence_type, e.source_database,
		             e.evidence_data, e.strength, e.confidence, e.created_at
		      FROM evidence e
		      WHERE e.variant_id = $1
		      ORDER BY e.strength DESC, e.confidence DESC`,
		Args:         []interface{}{variantID},
		UseCache:     true,
		UsePrepared:  true,
		QueryType:    "evidence_lookup",
		ResourceType: "evidence",
	}
	
	return qo.ExecuteQuery(ctx, query)
}

// ExecuteACMGRulesQuery executes optimized queries for ACMG rules resources
func (qo *QueryOptimizer) ExecuteACMGRulesQuery(ctx context.Context, category string) ([]QueryResult, error) {
	query := OptimizedQuery{
		SQL: `SELECT r.id, r.code, r.category, r.strength, r.description,
		             r.implementation_notes, r.evidence_requirements, r.created_at
		      FROM acmg_rules r
		      WHERE r.category = $1 OR $1 = ''
		      ORDER BY r.category, r.code`,
		Args:         []interface{}{category},
		UseCache:     true,
		UsePrepared:  true,
		QueryType:    "acmg_rules_lookup",
		ResourceType: "acmg_rules",
	}
	
	return qo.ExecuteQuery(ctx, query)
}

// ExecuteBatchVariantQuery executes optimized batch queries for multiple variants
func (qo *QueryOptimizer) ExecuteBatchVariantQuery(ctx context.Context, variantIDs []string) ([]QueryResult, error) {
	if len(variantIDs) == 0 {
		return []QueryResult{}, nil
	}

	// Build parameterized query for batch lookup
	placeholders := make([]string, len(variantIDs))
	args := make([]interface{}, len(variantIDs))
	for i, id := range variantIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	sql := fmt.Sprintf(`SELECT v.id, v.hgvs_notation, v.chromosome, v.position, v.ref_allele, v.alt_allele,
	                           v.gene_symbol, v.transcript_id, v.created_at, v.updated_at
	                    FROM variants v 
	                    WHERE v.id IN (%s)`, strings.Join(placeholders, ","))

	query := OptimizedQuery{
		SQL:          sql,
		Args:         args,
		UseCache:     true,
		UsePrepared:  false, // Don't use prepared statements for dynamic queries
		QueryType:    "batch_variant_lookup",
		ResourceType: "variant",
	}
	
	return qo.ExecuteQuery(ctx, query)
}

// GetQueryStats returns query performance statistics
func (qo *QueryOptimizer) GetQueryStats() QueryStats {
	qo.queryStatsMutex.RLock()
	defer qo.queryStatsMutex.RUnlock()
	return qo.queryStats
}

// ClearQueryCache clears all cached queries
func (qo *QueryOptimizer) ClearQueryCache() {
	qo.queryCacheMutex.Lock()
	defer qo.queryCacheMutex.Unlock()
	qo.queryCache = make(map[string]*CachedQuery)
}

// ClearPreparedStatements closes and clears all prepared statements
func (qo *QueryOptimizer) ClearPreparedStatements() error {
	qo.preparedStmtsMutex.Lock()
	defer qo.preparedStmtsMutex.Unlock()

	var errors []string
	for _, stmt := range qo.preparedStmts {
		if err := stmt.Close(); err != nil {
			errors = append(errors, err.Error())
		}
	}

	qo.preparedStmts = make(map[string]*sql.Stmt)

	if len(errors) > 0 {
		return fmt.Errorf("errors closing prepared statements: %v", strings.Join(errors, "; "))
	}
	return nil
}

// IsHealthy checks if the query optimizer is functioning properly
func (qo *QueryOptimizer) IsHealthy(ctx context.Context) bool {
	if qo.config.DB == nil {
		return false
	}

	// Test database connection
	if err := qo.config.DB.PingContext(ctx); err != nil {
		return false
	}

	// Test query execution
	query := OptimizedQuery{
		SQL:       "SELECT 1 as health_check",
		Args:      []interface{}{},
		UseCache:  false,
		QueryType: "health_check",
	}

	_, err := qo.ExecuteQuery(ctx, query)
	return err == nil
}

// Private helper methods

func (qo *QueryOptimizer) generateQueryKey(sql string, args []interface{}) string {
	key := sql
	for _, arg := range args {
		key += fmt.Sprintf("::%v", arg)
	}
	return key
}

func (qo *QueryOptimizer) getCachedQuery(key string) *CachedQuery {
	qo.queryCacheMutex.RLock()
	defer qo.queryCacheMutex.RUnlock()

	if cached, exists := qo.queryCache[key]; exists {
		if time.Now().Before(cached.ExpiresAt) {
			return cached
		}
		// Remove expired entry
		delete(qo.queryCache, key)
	}
	return nil
}

func (qo *QueryOptimizer) setCachedQuery(key string, cached *CachedQuery) {
	qo.queryCacheMutex.Lock()
	defer qo.queryCacheMutex.Unlock()
	qo.queryCache[key] = cached
}

func (qo *QueryOptimizer) getPreparedStatement(sql string) (*sql.Stmt, error) {
	qo.preparedStmtsMutex.RLock()
	if stmt, exists := qo.preparedStmts[sql]; exists {
		qo.preparedStmtsMutex.RUnlock()
		qo.queryStatsMutex.Lock()
		qo.queryStats.PreparedStmtHits++
		qo.queryStatsMutex.Unlock()
		return stmt, nil
	}
	qo.preparedStmtsMutex.RUnlock()

	// Create new prepared statement
	qo.preparedStmtsMutex.Lock()
	defer qo.preparedStmtsMutex.Unlock()

	// Double-check after acquiring write lock
	if stmt, exists := qo.preparedStmts[sql]; exists {
		qo.queryStats.PreparedStmtHits++
		return stmt, nil
	}

	// Check if we've reached the limit
	if len(qo.preparedStmts) >= qo.config.MaxPreparedStatements {
		// Remove oldest prepared statement (simple FIFO eviction)
		for key, stmt := range qo.preparedStmts {
			stmt.Close()
			delete(qo.preparedStmts, key)
			break
		}
	}

	stmt, err := qo.config.DB.Prepare(sql)
	if err != nil {
		qo.queryStatsMutex.Lock()
		qo.queryStats.PreparedStmtMiss++
		qo.queryStatsMutex.Unlock()
		return nil, err
	}

	qo.preparedStmts[sql] = stmt
	qo.queryStatsMutex.Lock()
	qo.queryStats.PreparedStmtMiss++
	qo.queryStatsMutex.Unlock()
	
	return stmt, nil
}

func (qo *QueryOptimizer) scanResults(rows *sql.Rows) ([]QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []QueryResult

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Convert to map for easier access
		data := make(map[string]interface{})
		for i, col := range columns {
			data[col] = values[i]
		}

		results = append(results, QueryResult{
			Columns: columns,
			Values:  values,
			Data:    data,
		})
	}

	return results, rows.Err()
}

func (qo *QueryOptimizer) updateQueryStats(queryType string, duration time.Duration, cached bool, preparedHit bool, failed bool) {
	qo.queryStatsMutex.Lock()
	defer qo.queryStatsMutex.Unlock()

	qo.queryStats.TotalQueries++
	if cached {
		qo.queryStats.CachedQueries++
	}
	if failed {
		qo.queryStats.FailedQueries++
	}
	if duration > qo.config.SlowQueryThreshold {
		qo.queryStats.SlowQueries++
	}

	// Update query type distribution
	qo.queryStats.QueryDistribution[queryType]++

	// Update average query time
	if qo.queryStats.TotalQueries == 1 {
		qo.queryStats.AverageQueryTime = duration
	} else {
		// Running average calculation
		oldAvg := qo.queryStats.AverageQueryTime
		newAvg := oldAvg + (duration-oldAvg)/time.Duration(qo.queryStats.TotalQueries)
		qo.queryStats.AverageQueryTime = newAvg
	}
}