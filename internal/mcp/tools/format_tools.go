package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/acmg-amp-mcp-server/internal/mcp/protocol"
)

// FormatReportTool implements the format_report MCP tool for converting reports to different formats
type FormatReportTool struct {
	logger *logrus.Logger
}

// FormatReportParams defines parameters for the format_report tool
type FormatReportParams struct {
	Report       ReportResult           `json:"report" validate:"required"`
	OutputFormat string                 `json:"output_format" validate:"required"`
	StyleOptions map[string]interface{} `json:"style_options,omitempty"`
	IncludeHeader bool                  `json:"include_header,omitempty"`
	IncludeFooter bool                  `json:"include_footer,omitempty"`
	PageBreaks    bool                  `json:"page_breaks,omitempty"`
	FontSize      string                `json:"font_size,omitempty"`
	Margins       string                `json:"margins,omitempty"`
}

// FormatReportResult contains the formatted report
type FormatReportResult struct {
	FormattedContent string                 `json:"formatted_content"`
	Format           string                 `json:"format"`
	Size             int                    `json:"size"`
	Encoding         string                 `json:"encoding"`
	Metadata         map[string]interface{} `json:"metadata"`
	ExportOptions    map[string]interface{} `json:"export_options,omitempty"`
}

// NewFormatReportTool creates a new format_report tool
func NewFormatReportTool(logger *logrus.Logger) *FormatReportTool {
	return &FormatReportTool{
		logger: logger,
	}
}

// HandleTool implements the ToolHandler interface for format_report
func (t *FormatReportTool) HandleTool(ctx context.Context, req *protocol.JSONRPC2Request) *protocol.JSONRPC2Response {
	t.logger.WithField("tool", "format_report").Info("Processing report formatting request")

	// Parse and validate parameters
	var params FormatReportParams
	if err := t.parseAndValidateParams(req.Params, &params); err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			},
		}
	}

	// Format the report
	result, err := t.formatReport(ctx, &params)
	if err != nil {
		return &protocol.JSONRPC2Response{
			Error: &protocol.RPCError{
				Code:    protocol.InternalError,
				Message: "Report formatting failed",
				Data:    err.Error(),
			},
		}
	}

	t.logger.WithFields(logrus.Fields{
		"report_id": params.Report.ReportID,
		"format":    params.OutputFormat,
		"size":      result.Size,
	}).Info("Report formatting completed")

	return &protocol.JSONRPC2Response{
		Result: map[string]interface{}{
			"formatted_report": result,
		},
	}
}

// GetToolInfo returns tool metadata
func (t *FormatReportTool) GetToolInfo() protocol.ToolInfo {
	return protocol.ToolInfo{
		Name:        "format_report",
		Description: "Format clinical reports into different output formats (JSON, text, HTML, PDF) with customizable styling",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"report": map[string]interface{}{
					"type":        "object",
					"description": "Report data from generate_report tool",
				},
				"output_format": map[string]interface{}{
					"type":        "string",
					"description": "Output format for the report",
					"enum":        []string{"json", "text", "html", "pdf", "markdown", "xml"},
				},
				"style_options": map[string]interface{}{
					"type":        "object",
					"description": "Styling options for the formatted output",
					"properties": map[string]interface{}{
						"theme": map[string]interface{}{
							"type": "string",
							"enum": []string{"clinical", "research", "minimal", "detailed"},
						},
						"colors": map[string]interface{}{
							"type":        "boolean",
							"description": "Include colors in output (where supported)",
						},
						"tables": map[string]interface{}{
							"type":        "boolean",
							"description": "Format data as tables where appropriate",
						},
					},
				},
				"include_header": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include report header with metadata",
				},
				"include_footer": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Include report footer with disclaimers",
				},
			},
			"required": []string{"report", "output_format"},
		},
	}
}

// ValidateParams validates tool parameters
func (t *FormatReportTool) ValidateParams(params interface{}) error {
	var formatParams FormatReportParams
	return t.parseAndValidateParams(params, &formatParams)
}

// parseAndValidateParams parses and validates input parameters
func (t *FormatReportTool) parseAndValidateParams(params interface{}, target *FormatReportParams) error {
	if params == nil {
		return fmt.Errorf("missing required parameters")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, target); err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Validate required fields
	if target.OutputFormat == "" {
		return fmt.Errorf("output_format is required")
	}

	// Validate that report is present
	if target.Report.ReportID == "" && target.Report.HGVSNotation == "" {
		return fmt.Errorf("report data is required")
	}

	// Validate format
	validFormats := []string{"json", "text", "html", "pdf", "markdown", "xml"}
	if !t.isValidFormat(target.OutputFormat, validFormats) {
		return fmt.Errorf("invalid output format: %s", target.OutputFormat)
	}

	// Set defaults
	if target.FontSize == "" {
		target.FontSize = "12pt"
	}

	if target.Margins == "" {
		target.Margins = "1in"
	}

	return nil
}

// formatReport formats the report according to the specified format
func (t *FormatReportTool) formatReport(ctx context.Context, params *FormatReportParams) (*FormatReportResult, error) {
	var content string
	var err error

	switch params.OutputFormat {
	case "json":
		content, err = t.formatAsJSON(params)
	case "text":
		content, err = t.formatAsText(params)
	case "html":
		content, err = t.formatAsHTML(params)
	case "pdf":
		content, err = t.formatAsPDF(params)
	case "markdown":
		content, err = t.formatAsMarkdown(params)
	case "xml":
		content, err = t.formatAsXML(params)
	default:
		return nil, fmt.Errorf("unsupported format: %s", params.OutputFormat)
	}

	if err != nil {
		return nil, fmt.Errorf("formatting failed: %w", err)
	}

	result := &FormatReportResult{
		FormattedContent: content,
		Format:           params.OutputFormat,
		Size:             len(content),
		Encoding:         "UTF-8",
		Metadata: map[string]interface{}{
			"report_id":       params.Report.ReportID,
			"generation_date": params.Report.GenerationDate,
			"format":          params.OutputFormat,
		},
	}

	return result, nil
}

// Format-specific implementations
func (t *FormatReportTool) formatAsJSON(params *FormatReportParams) (string, error) {
	jsonBytes, err := json.MarshalIndent(params.Report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON marshaling failed: %w", err)
	}
	return string(jsonBytes), nil
}

func (t *FormatReportTool) formatAsText(params *FormatReportParams) (string, error) {
	var sb strings.Builder

	if params.IncludeHeader {
		sb.WriteString(t.generateTextHeader(params))
		sb.WriteString("\n\n")
	}

	// Executive Summary
	sb.WriteString("EXECUTIVE SUMMARY\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n")
	sb.WriteString(fmt.Sprintf("Variant: %s\n", params.Report.HGVSNotation))
	sb.WriteString(fmt.Sprintf("Gene: %s\n", params.Report.GeneSymbol))
	sb.WriteString(fmt.Sprintf("Classification: %s\n", params.Report.Summary.Classification))
	sb.WriteString(fmt.Sprintf("Confidence: %.2f\n", params.Report.Summary.Confidence))
	sb.WriteString("\n")

	// Sections
	for sectionName, sectionData := range params.Report.Sections {
		sb.WriteString(strings.ToUpper(sectionName) + "\n")
		sb.WriteString(strings.Repeat("-", len(sectionName)) + "\n")
		sb.WriteString(t.formatSectionAsText(sectionData))
		sb.WriteString("\n\n")
	}

	// Recommendations
	sb.WriteString("RECOMMENDATIONS\n")
	sb.WriteString(strings.Repeat("-", 15) + "\n")
	for i, rec := range params.Report.Recommendations {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
	}
	sb.WriteString("\n")

	if params.IncludeFooter {
		sb.WriteString(t.generateTextFooter(params))
	}

	return sb.String(), nil
}

func (t *FormatReportTool) formatAsHTML(params *FormatReportParams) (string, error) {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString("<title>Clinical Report - " + params.Report.HGVSNotation + "</title>\n")
	sb.WriteString(t.getHTMLStyles(params))
	sb.WriteString("</head>\n<body>\n")

	if params.IncludeHeader {
		sb.WriteString(t.generateHTMLHeader(params))
	}

	// Executive Summary
	sb.WriteString("<div class='executive-summary'>\n")
	sb.WriteString("<h2>Executive Summary</h2>\n")
	sb.WriteString(fmt.Sprintf("<p><strong>Variant:</strong> %s</p>\n", params.Report.HGVSNotation))
	sb.WriteString(fmt.Sprintf("<p><strong>Gene:</strong> %s</p>\n", params.Report.GeneSymbol))
	sb.WriteString(fmt.Sprintf("<p><strong>Classification:</strong> <span class='classification'>%s</span></p>\n", params.Report.Summary.Classification))
	sb.WriteString(fmt.Sprintf("<p><strong>Confidence:</strong> %.2f</p>\n", params.Report.Summary.Confidence))
	sb.WriteString("</div>\n\n")

	// Sections
	for sectionName, sectionData := range params.Report.Sections {
		sb.WriteString(fmt.Sprintf("<div class='section' id='%s'>\n", sectionName))
		sb.WriteString(fmt.Sprintf("<h3>%s</h3>\n", t.titleCase(sectionName)))
		sb.WriteString(t.formatSectionAsHTML(sectionData))
		sb.WriteString("</div>\n\n")
	}

	// Recommendations
	sb.WriteString("<div class='recommendations'>\n")
	sb.WriteString("<h3>Recommendations</h3>\n<ol>\n")
	for _, rec := range params.Report.Recommendations {
		sb.WriteString(fmt.Sprintf("<li>%s</li>\n", rec))
	}
	sb.WriteString("</ol>\n</div>\n")

	if params.IncludeFooter {
		sb.WriteString(t.generateHTMLFooter(params))
	}

	sb.WriteString("</body>\n</html>")
	return sb.String(), nil
}

func (t *FormatReportTool) formatAsMarkdown(params *FormatReportParams) (string, error) {
	var sb strings.Builder

	if params.IncludeHeader {
		sb.WriteString(t.generateMarkdownHeader(params))
		sb.WriteString("\n\n")
	}

	// Executive Summary
	sb.WriteString("# Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Variant:** %s\n", params.Report.HGVSNotation))
	sb.WriteString(fmt.Sprintf("**Gene:** %s\n", params.Report.GeneSymbol))
	sb.WriteString(fmt.Sprintf("**Classification:** %s\n", params.Report.Summary.Classification))
	sb.WriteString(fmt.Sprintf("**Confidence:** %.2f\n\n", params.Report.Summary.Confidence))

	// Sections
	for sectionName, sectionData := range params.Report.Sections {
		sb.WriteString(fmt.Sprintf("## %s\n\n", t.titleCase(sectionName)))
		sb.WriteString(t.formatSectionAsMarkdown(sectionData))
		sb.WriteString("\n\n")
	}

	// Recommendations
	sb.WriteString("## Recommendations\n\n")
	for i, rec := range params.Report.Recommendations {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
	}
	sb.WriteString("\n")

	if params.IncludeFooter {
		sb.WriteString(t.generateMarkdownFooter(params))
	}

	return sb.String(), nil
}

func (t *FormatReportTool) formatAsPDF(params *FormatReportParams) (string, error) {
	// For now, return HTML that can be converted to PDF
	// In a real implementation, this would use a PDF library
	htmlContent, err := t.formatAsHTML(params)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("<!-- PDF Content (HTML for conversion) -->\n%s", htmlContent), nil
}

func (t *FormatReportTool) formatAsXML(params *FormatReportParams) (string, error) {
	var sb strings.Builder

	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<clinical_report>\n")
	sb.WriteString(fmt.Sprintf("  <report_id>%s</report_id>\n", params.Report.ReportID))
	sb.WriteString(fmt.Sprintf("  <variant>%s</variant>\n", params.Report.HGVSNotation))
	sb.WriteString(fmt.Sprintf("  <gene>%s</gene>\n", params.Report.GeneSymbol))
	sb.WriteString(fmt.Sprintf("  <generation_date>%s</generation_date>\n", params.Report.GenerationDate))

	sb.WriteString("  <summary>\n")
	sb.WriteString(fmt.Sprintf("    <classification>%s</classification>\n", params.Report.Summary.Classification))
	sb.WriteString(fmt.Sprintf("    <confidence>%.2f</confidence>\n", params.Report.Summary.Confidence))
	sb.WriteString("  </summary>\n")

	sb.WriteString("  <sections>\n")
	for sectionName, sectionData := range params.Report.Sections {
		sb.WriteString(fmt.Sprintf("    <section name=\"%s\">\n", sectionName))
		sb.WriteString(t.formatSectionAsXML(sectionData))
		sb.WriteString("    </section>\n")
	}
	sb.WriteString("  </sections>\n")

	sb.WriteString("</clinical_report>\n")
	return sb.String(), nil
}

// Helper methods for formatting
func (t *FormatReportTool) formatSectionAsText(data interface{}) string {
	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonBytes)
}

func (t *FormatReportTool) formatSectionAsHTML(data interface{}) string {
	if dataMap, ok := data.(map[string]interface{}); ok {
		var sb strings.Builder
		sb.WriteString("<div class='section-content'>\n")
		for key, value := range dataMap {
			sb.WriteString(fmt.Sprintf("<p><strong>%s:</strong> %v</p>\n", t.titleCase(key), value))
		}
		sb.WriteString("</div>\n")
		return sb.String()
	}
	return fmt.Sprintf("<pre>%v</pre>\n", data)
}

func (t *FormatReportTool) formatSectionAsMarkdown(data interface{}) string {
	if dataMap, ok := data.(map[string]interface{}); ok {
		var sb strings.Builder
		for key, value := range dataMap {
			sb.WriteString(fmt.Sprintf("**%s:** %v\n", t.titleCase(key), value))
		}
		return sb.String()
	}
	return fmt.Sprintf("```\n%v\n```", data)
}

func (t *FormatReportTool) formatSectionAsXML(data interface{}) string {
	if dataMap, ok := data.(map[string]interface{}); ok {
		var sb strings.Builder
		for key, value := range dataMap {
			sb.WriteString(fmt.Sprintf("      <%s>%v</%s>\n", key, value, key))
		}
		return sb.String()
	}
	return fmt.Sprintf("      <data>%v</data>\n", data)
}

// Header/Footer generators
func (t *FormatReportTool) generateTextHeader(params *FormatReportParams) string {
	return fmt.Sprintf("CLINICAL GENETIC VARIANT REPORT\nReport ID: %s\nGenerated: %s\n%s",
		params.Report.ReportID,
		params.Report.GenerationDate,
		strings.Repeat("=", 60))
}

func (t *FormatReportTool) generateTextFooter(params *FormatReportParams) string {
	var sb strings.Builder
	sb.WriteString("DISCLAIMERS\n")
	sb.WriteString(strings.Repeat("-", 11) + "\n")
	for _, disclaimer := range params.Report.Disclaimers {
		sb.WriteString("• " + disclaimer + "\n")
	}
	return sb.String()
}

func (t *FormatReportTool) generateHTMLHeader(params *FormatReportParams) string {
	return fmt.Sprintf("<header class='report-header'>\n<h1>Clinical Genetic Variant Report</h1>\n<p>Report ID: %s | Generated: %s</p>\n</header>\n",
		params.Report.ReportID,
		params.Report.GenerationDate)
}

func (t *FormatReportTool) generateHTMLFooter(params *FormatReportParams) string {
	var sb strings.Builder
	sb.WriteString("<footer class='disclaimers'>\n<h4>Disclaimers</h4>\n<ul>\n")
	for _, disclaimer := range params.Report.Disclaimers {
		sb.WriteString(fmt.Sprintf("<li>%s</li>\n", disclaimer))
	}
	sb.WriteString("</ul>\n</footer>\n")
	return sb.String()
}

func (t *FormatReportTool) generateMarkdownHeader(params *FormatReportParams) string {
	return fmt.Sprintf("# Clinical Genetic Variant Report\n\n**Report ID:** %s  \n**Generated:** %s",
		params.Report.ReportID,
		params.Report.GenerationDate)
}

func (t *FormatReportTool) generateMarkdownFooter(params *FormatReportParams) string {
	var sb strings.Builder
	sb.WriteString("## Disclaimers\n\n")
	for _, disclaimer := range params.Report.Disclaimers {
		sb.WriteString("- " + disclaimer + "\n")
	}
	return sb.String()
}

func (t *FormatReportTool) getHTMLStyles(params *FormatReportParams) string {
	return `<style>
body { font-family: Arial, sans-serif; line-height: 1.6; margin: 40px; }
.report-header { border-bottom: 2px solid #333; margin-bottom: 20px; }
.executive-summary { background: #f5f5f5; padding: 15px; margin: 20px 0; }
.classification { font-weight: bold; color: #d32f2f; }
.section { margin: 20px 0; }
.section h3 { color: #1976d2; border-bottom: 1px solid #ddd; }
.recommendations { background: #e3f2fd; padding: 15px; margin: 20px 0; }
.disclaimers { font-size: 0.9em; color: #666; margin-top: 30px; }
</style>`
}

// Utility methods
func (t *FormatReportTool) titleCase(s string) string {
	return strings.Title(strings.ReplaceAll(s, "_", " "))
}

func (t *FormatReportTool) isValidFormat(format string, validFormats []string) bool {
	for _, valid := range validFormats {
		if format == valid {
			return true
		}
	}
	return false
}