package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acmg-amp-mcp-server/internal/feedback"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// =============================================================================
// Submit Feedback Tests
// =============================================================================

func TestSubmitFeedbackTool_HandleTool_Success(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)
	tool := NewSubmitFeedbackTool(logger, store)

	params := map[string]interface{}{
		"variant":                   "BRCA1:c.5266dupC",
		"normalized_hgvs":           "NM_007294.4:c.5266dup",
		"cancer_type":               "breast",
		"suggested_classification":  "Pathogenic",
		"user_classification":       "Likely Pathogenic",
		"notes":                     "Additional family history",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "submit_feedback",
		Params:  params,
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	feedbackResult := resultMap["feedback"].(SubmitFeedbackResult)
	assert.True(t, feedbackResult.Success)
	assert.Contains(t, feedbackResult.Message, "corrected")
	assert.NotNil(t, feedbackResult.Feedback)
	assert.False(t, feedbackResult.Feedback.UserAgreed)
}

func TestSubmitFeedbackTool_HandleTool_UserAgrees(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)
	tool := NewSubmitFeedbackTool(logger, store)

	params := map[string]interface{}{
		"variant":                   "CFTR:c.1521_1523del",
		"suggested_classification":  "Pathogenic",
		"user_classification":       "Pathogenic",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "submit_feedback",
		Params:  params,
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	feedbackResult := resultMap["feedback"].(SubmitFeedbackResult)
	assert.True(t, feedbackResult.Success)
	assert.Contains(t, feedbackResult.Message, "agreed")
	assert.True(t, feedbackResult.Feedback.UserAgreed)
}

func TestSubmitFeedbackTool_GetToolInfo(t *testing.T) {
	logger, _ := test.NewNullLogger()
	tool := NewSubmitFeedbackTool(logger, nil)

	info := tool.GetToolInfo()

	assert.Equal(t, "submit_feedback", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.NotNil(t, info.InputSchema)
}

// =============================================================================
// Query Feedback Tests
// =============================================================================

func TestQueryFeedbackTool_HandleTool_Found(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)

	// Save some feedback first
	fb := &feedback.Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.4:c.5266dup",
		CancerType:              "breast",
		SuggestedClassification: feedback.ClassificationPathogenic,
		UserClassification:      feedback.ClassificationLikelyPathogenic,
		UserAgreed:              false,
	}
	err := store.Save(context.Background(), fb)
	require.NoError(t, err)

	tool := NewQueryFeedbackTool(logger, store)

	params := map[string]interface{}{
		"variant":     "NM_007294.4:c.5266dup",
		"cancer_type": "breast",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_feedback",
		Params:  params,
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	queryResult := resultMap["feedback_query"].(QueryFeedbackResult)
	assert.True(t, queryResult.Found)
	assert.NotNil(t, queryResult.Feedback)
	assert.Equal(t, feedback.ClassificationLikelyPathogenic, queryResult.Feedback.UserClassification)
}

func TestQueryFeedbackTool_HandleTool_NotFound(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)
	tool := NewQueryFeedbackTool(logger, store)

	params := map[string]interface{}{
		"variant": "NM_000000.0:c.1A>G",
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "query_feedback",
		Params:  params,
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	queryResult := resultMap["feedback_query"].(QueryFeedbackResult)
	assert.False(t, queryResult.Found)
	assert.Nil(t, queryResult.Feedback)
}

// =============================================================================
// Export Feedback Tests
// =============================================================================

func TestExportFeedbackTool_HandleTool_Success(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)

	// Save some feedback
	fb := &feedback.Feedback{
		Variant:                 "CFTR:c.1521_1523del",
		NormalizedHGVS:          "NM_000492.3:c.1521_1523del",
		SuggestedClassification: feedback.ClassificationPathogenic,
		UserClassification:      feedback.ClassificationPathogenic,
		UserAgreed:              true,
	}
	err := store.Save(context.Background(), fb)
	require.NoError(t, err)

	tmpDir, _ := os.MkdirTemp("", "export-test-*")
	defer os.RemoveAll(tmpDir)

	tool := NewExportFeedbackTool(logger, store, tmpDir)

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "export_feedback",
		Params:  map[string]interface{}{},
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	exportResult := resultMap["export"].(ExportFeedbackResult)
	assert.True(t, exportResult.Success)
	assert.Equal(t, int64(1), exportResult.Count)
	assert.NotEmpty(t, exportResult.FilePath)

	// Verify file exists
	_, err = os.Stat(exportResult.FilePath)
	assert.NoError(t, err)
}

// =============================================================================
// Import Feedback Tests
// =============================================================================

func TestImportFeedbackTool_HandleTool_Success(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)

	// Create JSON file to import
	tmpDir, _ := os.MkdirTemp("", "import-test-*")
	defer os.RemoveAll(tmpDir)

	jsonContent := `{
		"version": "1.0",
		"count": 1,
		"feedback": [
			{
				"variant": "TP53:p.R273H",
				"normalized_hgvs": "NM_000546.6:c.817C>A",
				"suggested_classification": "Pathogenic",
				"user_classification": "Pathogenic",
				"user_agreed": true
			}
		]
	}`

	filePath := filepath.Join(tmpDir, "import.json")
	err := os.WriteFile(filePath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	tool := NewImportFeedbackTool(logger, store)

	params := map[string]interface{}{
		"file_path": filePath,
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "import_feedback",
		Params:  params,
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	importResult := resultMap["import"].(ImportFeedbackResult)
	assert.True(t, importResult.Success)
	assert.Equal(t, 1, importResult.Imported)
	assert.Equal(t, 0, importResult.Skipped)

	// Verify import
	count, _ := store.Count(context.Background())
	assert.Equal(t, int64(1), count)
}

// =============================================================================
// List Feedback Tests
// =============================================================================

func TestListFeedbackTool_HandleTool_Success(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)

	// Save some feedback
	for i := 0; i < 3; i++ {
		fb := &feedback.Feedback{
			Variant:                 "variant" + string(rune('A'+i)),
			NormalizedHGVS:          "NM_00000" + string(rune('0'+i)) + ".1:c.1A>G",
			SuggestedClassification: feedback.ClassificationVUS,
			UserClassification:      feedback.ClassificationVUS,
			UserAgreed:              true,
		}
		err := store.Save(context.Background(), fb)
		require.NoError(t, err)
	}

	tool := NewListFeedbackTool(logger, store)

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "list_feedback",
		Params:  map[string]interface{}{},
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	listResult := resultMap["feedback_list"].(ListFeedbackResult)
	assert.Equal(t, int64(3), listResult.Total)
	assert.Len(t, listResult.Feedback, 3)
}

func TestListFeedbackTool_HandleTool_WithPagination(t *testing.T) {
	logger, _ := test.NewNullLogger()
	store := createTestFeedbackStore(t)

	// Save 5 entries
	for i := 0; i < 5; i++ {
		fb := &feedback.Feedback{
			Variant:                 "variant" + string(rune('A'+i)),
			NormalizedHGVS:          "NM_00000" + string(rune('0'+i)) + ".1:c.1A>G",
			SuggestedClassification: feedback.ClassificationVUS,
			UserClassification:      feedback.ClassificationVUS,
			UserAgreed:              true,
		}
		err := store.Save(context.Background(), fb)
		require.NoError(t, err)
	}

	tool := NewListFeedbackTool(logger, store)

	params := map[string]interface{}{
		"limit":  2,
		"offset": 1,
	}

	req := &protocol.JSONRPC2Request{
		JSONRPC: "2.0",
		Method:  "list_feedback",
		Params:  params,
		ID:      1,
	}

	// Act
	response := tool.HandleTool(context.Background(), req)

	// Assert
	assert.Nil(t, response.Error)
	resultMap := response.Result.(map[string]interface{})
	listResult := resultMap["feedback_list"].(ListFeedbackResult)
	assert.Equal(t, int64(5), listResult.Total)
	assert.Len(t, listResult.Feedback, 2)
	assert.Equal(t, 2, listResult.Limit)
	assert.Equal(t, 1, listResult.Offset)
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestFeedbackStore(t *testing.T) *feedback.SQLiteStore {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "feedback-tools-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := feedback.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		store.Close()
	})

	return store
}
