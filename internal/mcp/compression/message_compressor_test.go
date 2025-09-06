package compression

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageCompressor(t *testing.T) {
	config := CompressionConfig{}
	compressor := NewMessageCompressor(config)

	assert.NotNil(t, compressor)
	assert.Equal(t, CompressionGzip, compressor.config.DefaultType)
	assert.Equal(t, 1024, compressor.config.MinSize)
	assert.Equal(t, 6, compressor.config.Level)
}

func TestCompressMessageSmallData(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		MinSize: 1024,
	})

	smallData := []byte("small message")
	compressed, err := compressor.CompressMessage(smallData, "text/plain")

	require.NoError(t, err)
	assert.Equal(t, CompressionNone, compressed.Type)
	assert.Equal(t, smallData, compressed.Compressed)
	assert.Equal(t, len(smallData), compressed.OrigSize)
	assert.Equal(t, len(smallData), compressed.CompSize)
	assert.Equal(t, 1.0, compressed.Ratio)
}

func TestCompressMessageLargeData(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionGzip,
		MinSize:     100,
		Level:       6,
		EnableStats: true,
	})

	// Create large, compressible data
	largeData := []byte(strings.Repeat("This is a repeating pattern for compression testing. ", 100))
	
	compressed, err := compressor.CompressMessage(largeData, "text/plain")

	require.NoError(t, err)
	assert.Equal(t, CompressionGzip, compressed.Type)
	assert.Less(t, compressed.CompSize, compressed.OrigSize)
	assert.Less(t, compressed.Ratio, 1.0)
	assert.Equal(t, len(largeData), compressed.OrigSize)
}

func TestCompressDecompressRoundtrip(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionGzip,
		MinSize:     10,
		Level:       6,
	})

	originalData := []byte("This is test data that should be compressed and then decompressed back to the original form.")
	
	// Compress
	compressed, err := compressor.CompressMessage(originalData, "text/plain")
	require.NoError(t, err)

	// Decompress
	decompressed, err := compressor.DecompressMessage(compressed)
	require.NoError(t, err)

	assert.Equal(t, originalData, decompressed)
}

func TestCompressToolResult(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionGzip,
		MinSize:     50,
		EnableStats: true,
	})

	toolResult := map[string]interface{}{
		"classification": "Pathogenic",
		"confidence":     0.95,
		"acmg_criteria": []string{"PVS1", "PS1", "PM2"},
		"evidence": map[string]interface{}{
			"clinical":     "Strong pathogenic evidence from ClinVar",
			"functional":   "Loss of function confirmed",
			"computational": "Multiple prediction algorithms agree",
		},
	}

	compressed, err := compressor.CompressToolResult(toolResult)
	require.NoError(t, err)

	assert.Equal(t, "application/json+tool-result", compressed.ContentType)
	assert.Greater(t, compressed.OrigSize, 0)

	// Verify we can decompress back to original
	decompressed, err := compressor.DecompressMessage(compressed)
	require.NoError(t, err)

	var resultBack map[string]interface{}
	err = json.Unmarshal(decompressed, &resultBack)
	require.NoError(t, err)

	assert.Equal(t, "Pathogenic", resultBack["classification"])
	assert.Equal(t, 0.95, resultBack["confidence"])
}

func TestCompressResourceContent(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionZlib,
		MinSize:     10,
	})

	resourceContent := map[string]interface{}{
		"id":               "NM_000492.3:c.1521_1523delCTT",
		"hgvs_notation":    "NM_000492.3:c.1521_1523delCTT",
		"gene_symbol":      "CFTR",
		"chromosome":       "7",
		"position":         117199644,
		"classification":   "Pathogenic",
		"clinical_significance": "Disease causing",
	}

	compressed, err := compressor.CompressResourceContent(resourceContent, "variant")
	require.NoError(t, err)

	assert.Equal(t, "application/json+resource-variant", compressed.ContentType)
	
	// Verify decompression
	decompressed, err := compressor.DecompressMessage(compressed)
	require.NoError(t, err)

	var contentBack map[string]interface{}
	err = json.Unmarshal(decompressed, &contentBack)
	require.NoError(t, err)

	assert.Equal(t, "CFTR", contentBack["gene_symbol"])
	assert.Equal(t, "Pathogenic", contentBack["classification"])
}

func TestCompressBatchResponse(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionGzip,
		MinSize:     10,
	})

	batchResponses := []interface{}{
		map[string]interface{}{"id": 1, "result": "Pathogenic", "confidence": 0.95},
		map[string]interface{}{"id": 2, "result": "Benign", "confidence": 0.88},
		map[string]interface{}{"id": 3, "result": "VUS", "confidence": 0.45},
	}

	compressed, err := compressor.CompressBatchResponse(batchResponses)
	require.NoError(t, err)

	assert.Equal(t, "application/json+batch-response", compressed.ContentType)
	
	// Verify decompression
	decompressed, err := compressor.DecompressMessage(compressed)
	require.NoError(t, err)

	var responsesBack []interface{}
	err = json.Unmarshal(decompressed, &responsesBack)
	require.NoError(t, err)

	assert.Len(t, responsesBack, 3)
}

func TestCompressionStats(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionGzip,
		MinSize:     10,
		EnableStats: true,
	})

	// Reset stats
	compressor.ResetStats()

	testData := []byte(strings.Repeat("test data for compression ", 50))

	// Compress multiple messages
	_, err := compressor.CompressMessage(testData, "text/plain")
	require.NoError(t, err)

	_, err = compressor.CompressMessage([]byte("small"), "text/plain")
	require.NoError(t, err)

	stats := compressor.GetStats()
	assert.Equal(t, int64(2), stats.TotalMessages)
	assert.Equal(t, int64(1), stats.CompressedCount)
	assert.Equal(t, int64(1), stats.UncompressedCount)
	assert.Greater(t, stats.TotalOriginalSize, int64(0))
}

func TestCompressionTypes(t *testing.T) {
	testData := []byte(strings.Repeat("compression test data ", 100))

	// Test Gzip
	compressor := NewMessageCompressor(CompressionConfig{
		DefaultType: CompressionGzip,
		MinSize:     10,
	})

	gzipCompressed, err := compressor.CompressMessage(testData, "text/plain")
	require.NoError(t, err)
	assert.Equal(t, CompressionGzip, gzipCompressed.Type)

	gzipDecompressed, err := compressor.DecompressMessage(gzipCompressed)
	require.NoError(t, err)
	assert.Equal(t, testData, gzipDecompressed)

	// Test Zlib
	compressor.config.DefaultType = CompressionZlib
	zlibCompressed, err := compressor.CompressMessage(testData, "text/plain")
	require.NoError(t, err)
	assert.Equal(t, CompressionZlib, zlibCompressed.Type)

	zlibDecompressed, err := compressor.DecompressMessage(zlibCompressed)
	require.NoError(t, err)
	assert.Equal(t, testData, zlibDecompressed)
}

func TestContentTypeSettings(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		ContentTypeSettings: map[string]CompressionSettings{
			"custom/type": {
				Type:      CompressionZlib,
				Level:     9,
				Threshold: 50,
				Enabled:   true,
			},
		},
	})

	testData := []byte(strings.Repeat("custom content type test ", 10))

	compressed, err := compressor.CompressMessage(testData, "custom/type")
	require.NoError(t, err)

	// Should use the custom settings (zlib compression)
	assert.Equal(t, CompressionZlib, compressed.Type)
}

func TestShouldCompress(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		MinSize: 100,
		ContentTypeSettings: map[string]CompressionSettings{
			"special/type": {
				Type:      CompressionGzip,
				Threshold: 50,
				Enabled:   true,
			},
		},
	})

	// Below default threshold
	assert.False(t, compressor.ShouldCompress(50, "text/plain"))

	// Above default threshold
	assert.True(t, compressor.ShouldCompress(150, "text/plain"))

	// Custom threshold
	assert.True(t, compressor.ShouldCompress(60, "special/type"))
	assert.False(t, compressor.ShouldCompress(40, "special/type"))
}

func TestGetSupportedTypes(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{})
	
	supportedTypes := compressor.GetSupportedTypes()
	
	assert.Contains(t, supportedTypes, CompressionNone)
	assert.Contains(t, supportedTypes, CompressionGzip)
	assert.Contains(t, supportedTypes, CompressionZlib)
	assert.Len(t, supportedTypes, 3)
}

func TestConfigurePredefinedSettings(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{})
	
	compressor.ConfigurePredefinedSettings()
	
	settings := compressor.config.ContentTypeSettings
	assert.Contains(t, settings, "application/json")
	assert.Contains(t, settings, "application/json+tool-result")
	assert.Contains(t, settings, "application/json+resource-variant")
	assert.Contains(t, settings, "application/json+resource-evidence")

	// Verify tool-result uses higher compression
	toolResultSettings := settings["application/json+tool-result"]
	assert.Equal(t, 8, toolResultSettings.Level)

	// Verify evidence uses maximum compression
	evidenceSettings := settings["application/json+resource-evidence"]
	assert.Equal(t, 9, evidenceSettings.Level)
}

func TestEmptyData(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{})
	
	compressed, err := compressor.CompressMessage([]byte{}, "text/plain")
	require.NoError(t, err)

	assert.Equal(t, CompressionNone, compressed.Type)
	assert.Equal(t, 0, compressed.OrigSize)
	assert.Equal(t, 0, compressed.CompSize)
	assert.Equal(t, 1.0, compressed.Ratio)

	decompressed, err := compressor.DecompressMessage(compressed)
	require.NoError(t, err)
	assert.Empty(t, decompressed)
}

func TestCompressionRatioCalculation(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		MinSize:     10,
		EnableStats: true,
	})

	compressor.ResetStats()

	// Compress highly compressible data
	compressibleData := []byte(strings.Repeat("a", 1000))
	_, err := compressor.CompressMessage(compressibleData, "text/plain")
	require.NoError(t, err)

	ratio := compressor.GetCompressionRatio()
	assert.Less(t, ratio, 1.0)
	assert.Greater(t, ratio, 0.0)
}

func TestIsHealthy(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		MinSize: 10,
	})

	healthy := compressor.IsHealthy()
	assert.True(t, healthy)
}

func TestDecompressionSizeMismatch(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{})

	// Create a compressed message with incorrect original size
	validData := []byte("test data")
	compressed, err := compressor.CompressMessage(validData, "text/plain")
	require.NoError(t, err)

	// Tamper with original size
	compressed.OrigSize = 999

	// Decompression should fail
	_, err = compressor.DecompressMessage(compressed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

func TestPoorCompressionFallback(t *testing.T) {
	compressor := NewMessageCompressor(CompressionConfig{
		MinSize: 10,
	})

	// Use random-ish data that won't compress well
	randomData := []byte("abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+{}[]|\\:;\"'<>?,./")

	compressed, err := compressor.CompressMessage(randomData, "text/plain")
	require.NoError(t, err)

	// Should fall back to no compression due to poor ratio
	assert.Equal(t, CompressionNone, compressed.Type)
	assert.Equal(t, randomData, compressed.Compressed)
}