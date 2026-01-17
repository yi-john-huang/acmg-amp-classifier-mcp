package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/feedback"
	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// Error response helpers to reduce boilerplate
func invalidParamsError(msg string, data ...string) *protocol.JSONRPC2Response {
	resp := &protocol.JSONRPC2Response{
		Error: &protocol.RPCError{
			Code:    protocol.InvalidParams,
			Message: msg,
		},
	}
	if len(data) > 0 && data[0] != "" {
		resp.Error.Data = data[0]
	}
	return resp
}

func internalError(msg string, data string) *protocol.JSONRPC2Response {
	return &protocol.JSONRPC2Response{
		Error: &protocol.RPCError{
			Code:    protocol.InternalError,
			Message: msg,
			Data:    data,
		},
	}
}

// =============================================================================
// Submit Feedback Tool
// =============================================================================

// SubmitFeedbackTool implements the submit_feedback MCP tool
type SubmitFeedbackTool struct {
	logger *logrus.Logger
	store  feedback.Store
}

// SubmitFeedbackParams defines parameters for the submit_feedback tool
type SubmitFeedbackParams struct {
	Variant                 string `json:"variant"`
	NormalizedHGVS          string `json:"normalized_hgvs,omitempty"`
	CancerType              string `json:"cancer_type,omitempty"`
	SuggestedClassification string `json:"suggested_classification"`
	UserClassification      string `json:"user_classification"`
	EvidenceSummary         string `json:"evidence_summary,omitempty"`
	Notes                   string `json:"notes,omitempty"`
}

// SubmitFeedbackResult defines the result of submit_feedback
type SubmitFeedbackResult struct {
	Success  bool               `json:"success"`
	Message  string             `json:"message"`
	Feedback *feedback.Feedback `json:"feedback,omitempty"`
}

// NewSubmitFeedbackTool creates a new submit_feedback tool
func NewSubmitFeedbackTool(logger *logrus.Logger, store feedback.Store) *SubmitFeedbackTool {
	return &SubmitFeedbackTool{
		logger: logger,
		store:  store,
	}
}

// GetToolInfo returns the tool information for submit_feedback
func (t *SubmitFeedbackTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "submit_feedback",
		Description: "Submit user feedback on a variant classification. Stores the user's correction or agreement for future reference.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"variant": map[string]interface{}{
					"type":        "string",
					"description": "The variant notation (HGVS or gene symbol)",
				},
				"normalized_hgvs": map[string]interface{}{
					"type":        "string",
					"description": "Normalized HGVS notation (optional, auto-derived if not provided)",
				},
				"cancer_type": map[string]interface{}{
					"type":        "string",
					"description": "Clinical context or cancer type (optional)",
				},
				"suggested_classification": map[string]interface{}{
					"type":        "string",
					"description": "The system's suggested classification",
					"enum":        []string{"Pathogenic", "Likely Pathogenic", "VUS", "Likely Benign", "Benign"},
				},
				"user_classification": map[string]interface{}{
					"type":        "string",
					"description": "The user's classification (may be same as suggested or a correction)",
					"enum":        []string{"Pathogenic", "Likely Pathogenic", "VUS", "Likely Benign", "Benign"},
				},
				"evidence_summary": map[string]interface{}{
					"type":        "string",
					"description": "Summary of evidence used (optional)",
				},
				"notes": map[string]interface{}{
					"type":        "string",
					"description": "Additional notes or reasoning (optional)",
				},
			},
			"required": []string{"variant", "suggested_classification", "user_classification"},
		},
	}
}

// ValidateParams validates the input parameters
func (t *SubmitFeedbackTool) ValidateParams(params interface{}) error {
	var p SubmitFeedbackParams
	if err := ParseParams(params, &p); err != nil {
		return err
	}
	if p.Variant == "" {
		return fmt.Errorf("variant is required")
	}
	if p.SuggestedClassification == "" {
		return fmt.Errorf("suggested_classification is required")
	}
	if p.UserClassification == "" {
		return fmt.Errorf("user_classification is required")
	}
	return nil
}

// HandleTool handles the submit_feedback tool request
func (t *SubmitFeedbackTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	var params SubmitFeedbackParams
	if err := ParseParams(req.Params, &params); err != nil {
		return invalidParamsError("Invalid parameters", err.Error())
	}
	if err := t.ValidateParams(req.Params); err != nil {
		return invalidParamsError(err.Error())
	}

	normalizedHGVS := params.NormalizedHGVS
	if normalizedHGVS == "" {
		normalizedHGVS = params.Variant
	}

	userAgreed := params.SuggestedClassification == params.UserClassification
	fb := &feedback.Feedback{
		Variant:                 params.Variant,
		NormalizedHGVS:          normalizedHGVS,
		CancerType:              params.CancerType,
		SuggestedClassification: feedback.Classification(params.SuggestedClassification),
		UserClassification:      feedback.Classification(params.UserClassification),
		UserAgreed:              userAgreed,
		EvidenceSummary:         params.EvidenceSummary,
		Notes:                   params.Notes,
	}

	if err := t.store.Save(ctx, fb); err != nil {
		t.logger.WithError(err).Error("Failed to save feedback")
		return internalError("Failed to save feedback", err.Error())
	}

	msg := "Feedback saved: You agreed with the suggested classification"
	if !userAgreed {
		msg = fmt.Sprintf("Feedback saved: Classification corrected from %s to %s",
			params.SuggestedClassification, params.UserClassification)
	}

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"feedback": SubmitFeedbackResult{Success: true, Message: msg, Feedback: fb},
		},
	}
}

// =============================================================================
// Query Feedback Tool
// =============================================================================

// QueryFeedbackTool implements the query_feedback MCP tool
type QueryFeedbackTool struct {
	logger *logrus.Logger
	store  feedback.Store
}

// QueryFeedbackParams defines parameters for the query_feedback tool
type QueryFeedbackParams struct {
	Variant    string `json:"variant"`
	CancerType string `json:"cancer_type,omitempty"`
}

// QueryFeedbackResult defines the result of query_feedback
type QueryFeedbackResult struct {
	Found    bool               `json:"found"`
	Feedback *feedback.Feedback `json:"feedback,omitempty"`
	Message  string             `json:"message"`
}

// NewQueryFeedbackTool creates a new query_feedback tool
func NewQueryFeedbackTool(logger *logrus.Logger, store feedback.Store) *QueryFeedbackTool {
	return &QueryFeedbackTool{
		logger: logger,
		store:  store,
	}
}

// GetToolInfo returns the tool information for query_feedback
func (t *QueryFeedbackTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "query_feedback",
		Description: "Query previously saved user feedback for a variant. Returns the stored classification if available.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"variant": map[string]interface{}{
					"type":        "string",
					"description": "The variant notation (HGVS or normalized form)",
				},
				"cancer_type": map[string]interface{}{
					"type":        "string",
					"description": "Clinical context or cancer type (optional)",
				},
			},
			"required": []string{"variant"},
		},
	}
}

// ValidateParams validates the input parameters
func (t *QueryFeedbackTool) ValidateParams(params interface{}) error {
	var p QueryFeedbackParams
	if err := ParseParams(params, &p); err != nil {
		return err
	}
	if p.Variant == "" {
		return fmt.Errorf("variant is required")
	}
	return nil
}

// HandleTool handles the query_feedback tool request
func (t *QueryFeedbackTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	var params QueryFeedbackParams
	if err := ParseParams(req.Params, &params); err != nil {
		return invalidParamsError("Invalid parameters", err.Error())
	}
	if err := t.ValidateParams(req.Params); err != nil {
		return invalidParamsError(err.Error())
	}

	fb, err := t.store.Get(ctx, params.Variant, params.CancerType)
	if err != nil {
		t.logger.WithError(err).Error("Failed to query feedback")
		return internalError("Failed to query feedback", err.Error())
	}

	result := QueryFeedbackResult{Message: "No previous feedback found for this variant"}
	if fb != nil {
		result.Found = true
		result.Feedback = fb
		if fb.UserAgreed {
			result.Message = fmt.Sprintf("Found previous feedback: User agreed with %s classification", fb.UserClassification)
		} else {
			result.Message = fmt.Sprintf("Found previous feedback: User corrected to %s (was suggested %s)",
				fb.UserClassification, fb.SuggestedClassification)
		}
	}

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{"feedback_query": result},
	}
}

// =============================================================================
// Export Feedback Tool
// =============================================================================

// ExportFeedbackTool implements the export_feedback MCP tool
type ExportFeedbackTool struct {
	logger    *logrus.Logger
	store     feedback.Store
	exportDir string
}

// ExportFeedbackResult defines the result of export_feedback
type ExportFeedbackResult struct {
	Success  bool   `json:"success"`
	FilePath string `json:"file_path"`
	Count    int64  `json:"count"`
	Message  string `json:"message"`
}

// NewExportFeedbackTool creates a new export_feedback tool
func NewExportFeedbackTool(logger *logrus.Logger, store feedback.Store, exportDir string) *ExportFeedbackTool {
	return &ExportFeedbackTool{
		logger:    logger,
		store:     store,
		exportDir: exportDir,
	}
}

// GetToolInfo returns the tool information for export_feedback
func (t *ExportFeedbackTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "export_feedback",
		Description: "Export all saved feedback to a JSON file for backup.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

// ValidateParams validates the input parameters
func (t *ExportFeedbackTool) ValidateParams(params interface{}) error {
	return nil // No required parameters
}

// HandleTool handles the export_feedback tool request
func (t *ExportFeedbackTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	if err := os.MkdirAll(t.exportDir, 0755); err != nil {
		return internalError("Failed to create export directory", err.Error())
	}

	filename := fmt.Sprintf("feedback_export_%s.json", time.Now().Format("20060102_150405"))
	filePath := filepath.Join(t.exportDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return internalError("Failed to create export file", err.Error())
	}
	defer file.Close()

	if err := t.store.ExportJSON(ctx, file); err != nil {
		t.logger.WithError(err).Error("Failed to export feedback")
		return internalError("Failed to export feedback", err.Error())
	}

	count, _ := t.store.Count(ctx)
	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"export": ExportFeedbackResult{
				Success: true, FilePath: filePath, Count: count,
				Message: fmt.Sprintf("Exported %d feedback entries to %s", count, filePath),
			},
		},
	}
}

// =============================================================================
// Import Feedback Tool
// =============================================================================

// ImportFeedbackTool implements the import_feedback MCP tool
type ImportFeedbackTool struct {
	logger *logrus.Logger
	store  feedback.Store
}

// ImportFeedbackParams defines parameters for the import_feedback tool
type ImportFeedbackParams struct {
	FilePath string `json:"file_path"`
}

// ImportFeedbackResult defines the result of import_feedback
type ImportFeedbackResult struct {
	Success  bool   `json:"success"`
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Message  string `json:"message"`
}

// NewImportFeedbackTool creates a new import_feedback tool
func NewImportFeedbackTool(logger *logrus.Logger, store feedback.Store) *ImportFeedbackTool {
	return &ImportFeedbackTool{
		logger: logger,
		store:  store,
	}
}

// GetToolInfo returns the tool information for import_feedback
func (t *ImportFeedbackTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "import_feedback",
		Description: "Import feedback from a JSON backup file. Skips duplicates.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the JSON file to import",
				},
			},
			"required": []string{"file_path"},
		},
	}
}

// ValidateParams validates the input parameters
func (t *ImportFeedbackTool) ValidateParams(params interface{}) error {
	var p ImportFeedbackParams
	if err := ParseParams(params, &p); err != nil {
		return err
	}
	if p.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}
	return nil
}

// HandleTool handles the import_feedback tool request
func (t *ImportFeedbackTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	var params ImportFeedbackParams
	if err := ParseParams(req.Params, &params); err != nil {
		return invalidParamsError("Invalid parameters", err.Error())
	}
	if err := t.ValidateParams(req.Params); err != nil {
		return invalidParamsError(err.Error())
	}

	file, err := os.Open(params.FilePath)
	if err != nil {
		return invalidParamsError("Failed to open file", err.Error())
	}
	defer file.Close()

	imported, skipped, err := t.store.ImportJSON(ctx, file)
	if err != nil {
		t.logger.WithError(err).Error("Failed to import feedback")
		return internalError("Failed to import feedback", err.Error())
	}

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"import": ImportFeedbackResult{
				Success: true, Imported: imported, Skipped: skipped,
				Message: fmt.Sprintf("Imported %d entries, skipped %d duplicates", imported, skipped),
			},
		},
	}
}

// =============================================================================
// List Feedback Tool
// =============================================================================

// ListFeedbackTool implements the list_feedback MCP tool
type ListFeedbackTool struct {
	logger *logrus.Logger
	store  feedback.Store
}

// ListFeedbackParams defines parameters for the list_feedback tool
type ListFeedbackParams struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ListFeedbackResult defines the result of list_feedback
type ListFeedbackResult struct {
	Feedback []*feedback.Feedback `json:"feedback"`
	Total    int64                `json:"total"`
	Limit    int                  `json:"limit"`
	Offset   int                  `json:"offset"`
}

// NewListFeedbackTool creates a new list_feedback tool
func NewListFeedbackTool(logger *logrus.Logger, store feedback.Store) *ListFeedbackTool {
	return &ListFeedbackTool{
		logger: logger,
		store:  store,
	}
}

// GetToolInfo returns the tool information for list_feedback
func (t *ListFeedbackTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "list_feedback",
		Description: "List all saved feedback entries with pagination.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of entries to return (default 50)",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Number of entries to skip (default 0)",
				},
			},
		},
	}
}

// ValidateParams validates the input parameters
func (t *ListFeedbackTool) ValidateParams(params interface{}) error {
	return nil // No required parameters
}

// HandleTool handles the list_feedback tool request
func (t *ListFeedbackTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	var params ListFeedbackParams
	_ = ParseParams(req.Params, &params)

	limit, offset := params.Limit, params.Offset
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	feedbackList, err := t.store.List(ctx, limit, offset)
	if err != nil {
		t.logger.WithError(err).Error("Failed to list feedback")
		return internalError("Failed to list feedback", err.Error())
	}

	total, _ := t.store.Count(ctx)
	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"feedback_list": ListFeedbackResult{Feedback: feedbackList, Total: total, Limit: limit, Offset: offset},
		},
	}
}
