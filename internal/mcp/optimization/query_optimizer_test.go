package optimization

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func TestNewQueryOptimizer(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	config := QueryOptimizerConfig{
		DB:                       db,
		EnableQueryCache:         true,
		EnablePreparedStatements: true,
		EnableQueryStats:         true,
	}

	optimizer := NewQueryOptimizer(config)

	assert.NotNil(t, optimizer)
	assert.Equal(t, 10*time.Minute, optimizer.config.QueryCacheTTL)
	assert.Equal(t, 100, optimizer.config.MaxPreparedStatements)
	assert.Equal(t, time.Second, optimizer.config.SlowQueryThreshold)
}

func TestExecuteQuery(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:               db,
		EnableQueryCache: false,
		EnableQueryStats: true,
	})

	// Mock the query
	rows := sqlmock.NewRows([]string{"id", "name", "value"}).
		AddRow(1, "test", "result")

	mock.ExpectQuery("SELECT (.+) FROM test WHERE id = ?").
		WithArgs(1).
		WillReturnRows(rows)

	query := OptimizedQuery{
		SQL:       "SELECT id, name, value FROM test WHERE id = ?",
		Args:      []interface{}{1},
		UseCache:  false,
		QueryType: "test_query",
	}

	results, err := optimizer.ExecuteQuery(context.Background(), query)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, []string{"id", "name", "value"}, results[0].Columns)
	assert.Equal(t, int64(1), results[0].Data["id"])
	assert.Equal(t, "test", results[0].Data["name"])
	assert.Equal(t, "result", results[0].Data["value"])

	// Check stats
	stats := optimizer.GetQueryStats()
	assert.Equal(t, int64(1), stats.TotalQueries)
	assert.Equal(t, int64(0), stats.CachedQueries)
	assert.Equal(t, int64(1), stats.QueryDistribution["test_query"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteQueryWithCache(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:                db,
		EnableQueryCache:  true,
		QueryCacheTTL:     time.Minute,
		EnableQueryStats:  true,
	})

	// Mock the query once
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "cached_result")

	mock.ExpectQuery("SELECT (.+) FROM test WHERE id = ?").
		WithArgs(1).
		WillReturnRows(rows)

	query := OptimizedQuery{
		SQL:       "SELECT id, name FROM test WHERE id = ?",
		Args:      []interface{}{1},
		UseCache:  true,
		QueryType: "cached_query",
	}

	// First execution - should hit database
	results1, err := optimizer.ExecuteQuery(context.Background(), query)
	require.NoError(t, err)
	assert.Len(t, results1, 1)
	assert.Equal(t, "cached_result", results1[0].Data["name"])

	// Second execution - should hit cache
	results2, err := optimizer.ExecuteQuery(context.Background(), query)
	require.NoError(t, err)
	assert.Len(t, results2, 1)
	assert.Equal(t, "cached_result", results2[0].Data["name"])

	// Check stats
	stats := optimizer.GetQueryStats()
	assert.Equal(t, int64(2), stats.TotalQueries)
	assert.Equal(t, int64(1), stats.CachedQueries)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteQueryWithPreparedStatements(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:                       db,
		EnablePreparedStatements: true,
		EnableQueryStats:         true,
	})

	sql := "SELECT id, name FROM test WHERE id = ?"
	
	// Mock prepare statement
	mock.ExpectPrepare(sql)
	
	// Mock query execution
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "prepared_result")
	mock.ExpectQuery(sql).WithArgs(1).WillReturnRows(rows)

	query := OptimizedQuery{
		SQL:         sql,
		Args:        []interface{}{1},
		UsePrepared: true,
		QueryType:   "prepared_query",
	}

	results, err := optimizer.ExecuteQuery(context.Background(), query)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "prepared_result", results[0].Data["name"])

	// Check stats
	stats := optimizer.GetQueryStats()
	assert.Equal(t, int64(1), stats.TotalQueries)
	assert.Equal(t, int64(1), stats.PreparedStmtMiss) // First time creating the prepared statement

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteVariantQuery(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:                       db,
		EnableQueryCache:         true,
		EnablePreparedStatements: true,
	})

	variantSQL := `SELECT v.id, v.hgvs_notation, v.chromosome, v.position, v.ref_allele, v.alt_allele,
		             v.gene_symbol, v.transcript_id, v.created_at, v.updated_at
		      FROM variants v 
		      WHERE v.id = \$1 OR v.hgvs_notation = \$1`

	// Mock prepare statement
	mock.ExpectPrepare(variantSQL)

	// Mock query execution
	rows := sqlmock.NewRows([]string{"id", "hgvs_notation", "chromosome", "position", "ref_allele", "alt_allele", "gene_symbol", "transcript_id", "created_at", "updated_at"}).
		AddRow("var1", "NM_000492.3:c.1521_1523delCTT", "7", 117199644, "CTT", "", "CFTR", "NM_000492.3", time.Now(), time.Now())

	mock.ExpectQuery(variantSQL).
		WithArgs("var1").
		WillReturnRows(rows)

	results, err := optimizer.ExecuteVariantQuery(context.Background(), "var1")

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "var1", results[0].Data["id"])
	assert.Equal(t, "NM_000492.3:c.1521_1523delCTT", results[0].Data["hgvs_notation"])
	assert.Equal(t, "CFTR", results[0].Data["gene_symbol"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteInterpretationQuery(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:                       db,
		EnableQueryCache:         true,
		EnablePreparedStatements: true,
	})

	interpretationSQL := `SELECT i.id, i.variant_id, i.classification, i.confidence_score,
		             i.acmg_criteria, i.evidence_summary, i.clinical_significance,
		             i.created_at, i.updated_at, i.created_by
		      FROM interpretations i
		      WHERE i.id = \$1`

	// Mock prepare statement
	mock.ExpectPrepare(interpretationSQL)

	// Mock query execution
	rows := sqlmock.NewRows([]string{"id", "variant_id", "classification", "confidence_score", "acmg_criteria", "evidence_summary", "clinical_significance", "created_at", "updated_at", "created_by"}).
		AddRow("int1", "var1", "Pathogenic", 0.95, "PVS1,PS1", "Strong evidence summary", "Pathogenic", time.Now(), time.Now(), "analyst1")

	mock.ExpectQuery(interpretationSQL).
		WithArgs("int1").
		WillReturnRows(rows)

	results, err := optimizer.ExecuteInterpretationQuery(context.Background(), "int1")

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "int1", results[0].Data["id"])
	assert.Equal(t, "Pathogenic", results[0].Data["classification"])
	assert.Equal(t, 0.95, results[0].Data["confidence_score"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteBatchVariantQuery(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:               db,
		EnableQueryCache: true,
	})

	variantIDs := []string{"var1", "var2", "var3"}
	batchSQL := `SELECT v.id, v.hgvs_notation, v.chromosome, v.position, v.ref_allele, v.alt_allele,
	                           v.gene_symbol, v.transcript_id, v.created_at, v.updated_at
	                    FROM variants v 
	                    WHERE v.id IN \(\$1,\$2,\$3\)`

	// Mock query execution
	rows := sqlmock.NewRows([]string{"id", "hgvs_notation", "chromosome", "position", "ref_allele", "alt_allele", "gene_symbol", "transcript_id", "created_at", "updated_at"}).
		AddRow("var1", "NM_000492.3:c.1521_1523delCTT", "7", 117199644, "CTT", "", "CFTR", "NM_000492.3", time.Now(), time.Now()).
		AddRow("var2", "NM_000492.3:c.1624G>T", "7", 117199747, "G", "T", "CFTR", "NM_000492.3", time.Now(), time.Now()).
		AddRow("var3", "NM_000492.3:c.1652G>A", "7", 117199775, "G", "A", "CFTR", "NM_000492.3", time.Now(), time.Now())

	mock.ExpectQuery(batchSQL).
		WithArgs("var1", "var2", "var3").
		WillReturnRows(rows)

	results, err := optimizer.ExecuteBatchVariantQuery(context.Background(), variantIDs)

	require.NoError(t, err)
	assert.Len(t, results, 3)
	
	ids := make([]string, len(results))
	for i, result := range results {
		ids[i] = result.Data["id"].(string)
	}
	assert.Contains(t, ids, "var1")
	assert.Contains(t, ids, "var2")
	assert.Contains(t, ids, "var3")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryStatsTracking(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:                    db,
		EnableQueryStats:      true,
		SlowQueryThreshold:    100 * time.Millisecond,
	})

	// Mock a slow query
	rows := sqlmock.NewRows([]string{"result"}).AddRow("slow")
	mock.ExpectQuery("SELECT (.+)").
		WillDelayFor(150*time.Millisecond).
		WillReturnRows(rows)

	query := OptimizedQuery{
		SQL:       "SELECT * FROM slow_table",
		Args:      []interface{}{},
		QueryType: "slow_query",
	}

	_, err := optimizer.ExecuteQuery(context.Background(), query)
	require.NoError(t, err)

	stats := optimizer.GetQueryStats()
	assert.Equal(t, int64(1), stats.TotalQueries)
	assert.Equal(t, int64(1), stats.SlowQueries)
	assert.Greater(t, stats.AverageQueryTime, 100*time.Millisecond)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryCacheExpiration(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:               db,
		EnableQueryCache: true,
		QueryCacheTTL:    50 * time.Millisecond,
	})

	// Mock the query twice (cache should expire)
	rows1 := sqlmock.NewRows([]string{"result"}).AddRow("first")
	rows2 := sqlmock.NewRows([]string{"result"}).AddRow("second")
	
	mock.ExpectQuery("SELECT (.+)").WillReturnRows(rows1)
	mock.ExpectQuery("SELECT (.+)").WillReturnRows(rows2)

	query := OptimizedQuery{
		SQL:       "SELECT result FROM test",
		Args:      []interface{}{},
		UseCache:  true,
		QueryType: "expiry_test",
	}

	// First execution
	results1, err := optimizer.ExecuteQuery(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, "first", results1[0].Data["result"])

	// Wait for cache to expire
	time.Sleep(60 * time.Millisecond)

	// Second execution should hit database again
	results2, err := optimizer.ExecuteQuery(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, "second", results2[0].Data["result"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClearQueryCache(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:               db,
		EnableQueryCache: true,
	})

	// Manually add a cache entry
	optimizer.setCachedQuery("test_key", &CachedQuery{
		Query:     "SELECT * FROM test",
		Result:    []QueryResult{},
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	// Verify cache has entry
	assert.NotNil(t, optimizer.getCachedQuery("test_key"))

	// Clear cache
	optimizer.ClearQueryCache()

	// Verify cache is empty
	assert.Nil(t, optimizer.getCachedQuery("test_key"))
}

func TestIsHealthy(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB: db,
	})

	// Mock ping and health check query
	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"health_check"}).AddRow(1)
	mock.ExpectQuery("SELECT 1 as health_check").WillReturnRows(rows)

	healthy := optimizer.IsHealthy(context.Background())
	assert.True(t, healthy)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPreparedStatementEviction(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB:                       db,
		EnablePreparedStatements: true,
		MaxPreparedStatements:    2, // Small limit to test eviction
	})

	// Prepare 3 statements (should evict the first one)
	sql1 := "SELECT * FROM table1 WHERE id = ?"
	sql2 := "SELECT * FROM table2 WHERE id = ?"
	sql3 := "SELECT * FROM table3 WHERE id = ?"

	mock.ExpectPrepare(sql1)
	mock.ExpectPrepare(sql2)
	mock.ExpectPrepare(sql3).WillReturnCloseError(nil)

	_, err1 := optimizer.getPreparedStatement(sql1)
	require.NoError(t, err1)

	_, err2 := optimizer.getPreparedStatement(sql2)
	require.NoError(t, err2)

	// This should evict the first statement
	_, err3 := optimizer.getPreparedStatement(sql3)
	require.NoError(t, err3)

	// Verify only 2 statements are cached
	optimizer.preparedStmtsMutex.RLock()
	assert.Len(t, optimizer.preparedStmts, 2)
	optimizer.preparedStmtsMutex.RUnlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEmptyBatchVariantQuery(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	optimizer := NewQueryOptimizer(QueryOptimizerConfig{
		DB: db,
	})

	results, err := optimizer.ExecuteBatchVariantQuery(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, results)
}