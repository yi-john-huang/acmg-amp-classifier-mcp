package compression

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// CompressionType represents the type of compression algorithm
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
	CompressionZlib CompressionType = "zlib"
	CompressionLZ4  CompressionType = "lz4"
)

// CompressionConfig defines configuration for message compression
type CompressionConfig struct {
	// Default compression type
	DefaultType CompressionType
	// Minimum size threshold for compression (bytes)
	MinSize int
	// Compression level (0-9 for gzip/zlib)
	Level int
	// Enable compression statistics
	EnableStats bool
	// Content type specific settings
	ContentTypeSettings map[string]CompressionSettings
}

// CompressionSettings defines compression settings for specific content types
type CompressionSettings struct {
	Type      CompressionType `json:"type"`
	Level     int             `json:"level"`
	Threshold int             `json:"threshold"`
	Enabled   bool            `json:"enabled"`
}

// CompressedMessage represents a compressed JSON-RPC message
type CompressedMessage struct {
	Original    []byte          `json:"-"`
	Compressed  []byte          `json:"data"`
	Type        CompressionType `json:"compression_type"`
	OrigSize    int             `json:"original_size"`
	CompSize    int             `json:"compressed_size"`
	Ratio       float64         `json:"compression_ratio"`
	ContentType string          `json:"content_type"`
}

// MessageCompressor handles compression of JSON-RPC messages and large payloads
type MessageCompressor struct {
	config CompressionConfig
	stats  CompressionStats
	mutex  sync.RWMutex
}

// CompressionStats tracks compression performance metrics
type CompressionStats struct {
	TotalMessages     int64   `json:"total_messages"`
	CompressedCount   int64   `json:"compressed_count"`
	UncompressedCount int64   `json:"uncompressed_count"`
	TotalOriginalSize int64   `json:"total_original_size"`
	TotalCompSize     int64   `json:"total_compressed_size"`
	AverageRatio      float64 `json:"average_compression_ratio"`
	CompressionTime   int64   `json:"total_compression_time_ms"`
	DecompressionTime int64   `json:"total_decompression_time_ms"`
	TypeStats         map[CompressionType]TypeStats `json:"type_stats"`
}

// TypeStats tracks statistics for specific compression types
type TypeStats struct {
	Count         int64   `json:"count"`
	TotalOrigSize int64   `json:"total_original_size"`
	TotalCompSize int64   `json:"total_compressed_size"`
	AverageRatio  float64 `json:"average_ratio"`
	TotalTime     int64   `json:"total_time_ms"`
}

// NewMessageCompressor creates a new message compressor instance
func NewMessageCompressor(config CompressionConfig) *MessageCompressor {
	if config.DefaultType == "" {
		config.DefaultType = CompressionGzip
	}
	if config.MinSize == 0 {
		config.MinSize = 1024 // 1KB default threshold
	}
	if config.Level == 0 {
		config.Level = 6 // Default compression level
	}
	if config.ContentTypeSettings == nil {
		config.ContentTypeSettings = make(map[string]CompressionSettings)
	}

	return &MessageCompressor{
		config: config,
		stats: CompressionStats{
			TypeStats: make(map[CompressionType]TypeStats),
		},
	}
}

// CompressMessage compresses a JSON-RPC message or large payload
func (mc *MessageCompressor) CompressMessage(data []byte, contentType string) (*CompressedMessage, error) {
	if len(data) == 0 {
		return &CompressedMessage{
			Original:    data,
			Compressed:  data,
			Type:        CompressionNone,
			OrigSize:    0,
			CompSize:    0,
			Ratio:       1.0,
			ContentType: contentType,
		}, nil
	}

	// Determine compression settings for content type
	settings := mc.getSettingsForContentType(contentType)
	
	// Check if compression should be applied
	if !settings.Enabled || len(data) < settings.Threshold {
		mc.updateStats(CompressionNone, len(data), len(data), 0)
		return &CompressedMessage{
			Original:    data,
			Compressed:  data,
			Type:        CompressionNone,
			OrigSize:    len(data),
			CompSize:    len(data),
			Ratio:       1.0,
			ContentType: contentType,
		}, nil
	}

	// Compress the data
	compressed, err := mc.compress(data, settings.Type, settings.Level)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	ratio := float64(len(compressed)) / float64(len(data))

	// If compression doesn't provide significant benefit, return uncompressed
	if ratio > 0.9 {
		mc.updateStats(CompressionNone, len(data), len(data), 0)
		return &CompressedMessage{
			Original:    data,
			Compressed:  data,
			Type:        CompressionNone,
			OrigSize:    len(data),
			CompSize:    len(data),
			Ratio:       1.0,
			ContentType: contentType,
		}, nil
	}

	mc.updateStats(settings.Type, len(data), len(compressed), 0)

	return &CompressedMessage{
		Original:    data,
		Compressed:  compressed,
		Type:        settings.Type,
		OrigSize:    len(data),
		CompSize:    len(compressed),
		Ratio:       ratio,
		ContentType: contentType,
	}, nil
}

// DecompressMessage decompresses a compressed message
func (mc *MessageCompressor) DecompressMessage(compressed *CompressedMessage) ([]byte, error) {
	if compressed.Type == CompressionNone {
		return compressed.Compressed, nil
	}

	decompressed, err := mc.decompress(compressed.Compressed, compressed.Type)
	if err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	// Verify decompressed size matches original
	if len(decompressed) != compressed.OrigSize {
		return nil, fmt.Errorf("decompression size mismatch: expected %d, got %d", 
			compressed.OrigSize, len(decompressed))
	}

	return decompressed, nil
}

// CompressToolResult compresses a tool execution result
func (mc *MessageCompressor) CompressToolResult(result interface{}) (*CompressedMessage, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}

	return mc.CompressMessage(data, "application/json+tool-result")
}

// CompressResourceContent compresses MCP resource content
func (mc *MessageCompressor) CompressResourceContent(content interface{}, resourceType string) (*CompressedMessage, error) {
	data, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource content: %w", err)
	}

	contentType := fmt.Sprintf("application/json+resource-%s", resourceType)
	return mc.CompressMessage(data, contentType)
}

// CompressBatchResponse compresses a batch response containing multiple results
func (mc *MessageCompressor) CompressBatchResponse(responses []interface{}) (*CompressedMessage, error) {
	data, err := json.Marshal(responses)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch response: %w", err)
	}

	return mc.CompressMessage(data, "application/json+batch-response")
}

// GetStats returns compression performance statistics
func (mc *MessageCompressor) GetStats() CompressionStats {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.stats
}

// ResetStats resets compression statistics
func (mc *MessageCompressor) ResetStats() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.stats = CompressionStats{
		TypeStats: make(map[CompressionType]TypeStats),
	}
}

// GetCompressionRatio returns the overall compression ratio
func (mc *MessageCompressor) GetCompressionRatio() float64 {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	if mc.stats.TotalOriginalSize == 0 {
		return 1.0
	}
	return float64(mc.stats.TotalCompSize) / float64(mc.stats.TotalOriginalSize)
}

// ShouldCompress determines if data should be compressed based on size and type
func (mc *MessageCompressor) ShouldCompress(dataSize int, contentType string) bool {
	settings := mc.getSettingsForContentType(contentType)
	return settings.Enabled && dataSize >= settings.Threshold
}

// GetSupportedTypes returns list of supported compression types
func (mc *MessageCompressor) GetSupportedTypes() []CompressionType {
	return []CompressionType{
		CompressionNone,
		CompressionGzip,
		CompressionZlib,
	}
}

// Private helper methods

func (mc *MessageCompressor) getSettingsForContentType(contentType string) CompressionSettings {
	if settings, exists := mc.config.ContentTypeSettings[contentType]; exists {
		return settings
	}

	// Return default settings
	return CompressionSettings{
		Type:      mc.config.DefaultType,
		Level:     mc.config.Level,
		Threshold: mc.config.MinSize,
		Enabled:   true,
	}
}

func (mc *MessageCompressor) compress(data []byte, compressionType CompressionType, level int) ([]byte, error) {
	switch compressionType {
	case CompressionGzip:
		return mc.compressGzip(data, level)
	case CompressionZlib:
		return mc.compressZlib(data, level)
	case CompressionNone:
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", compressionType)
	}
}

func (mc *MessageCompressor) decompress(data []byte, compressionType CompressionType) ([]byte, error) {
	switch compressionType {
	case CompressionGzip:
		return mc.decompressGzip(data)
	case CompressionZlib:
		return mc.decompressZlib(data)
	case CompressionNone:
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", compressionType)
	}
}

func (mc *MessageCompressor) compressGzip(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (mc *MessageCompressor) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func (mc *MessageCompressor) compressZlib(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (mc *MessageCompressor) decompressZlib(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func (mc *MessageCompressor) updateStats(compressionType CompressionType, originalSize, compressedSize int, processingTime int64) {
	if !mc.config.EnableStats {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.stats.TotalMessages++
	mc.stats.TotalOriginalSize += int64(originalSize)
	mc.stats.TotalCompSize += int64(compressedSize)

	if compressionType == CompressionNone {
		mc.stats.UncompressedCount++
	} else {
		mc.stats.CompressedCount++
	}

	// Update average compression ratio
	if mc.stats.TotalOriginalSize > 0 {
		mc.stats.AverageRatio = float64(mc.stats.TotalCompSize) / float64(mc.stats.TotalOriginalSize)
	}

	// Update type-specific stats
	typeStats := mc.stats.TypeStats[compressionType]
	typeStats.Count++
	typeStats.TotalOrigSize += int64(originalSize)
	typeStats.TotalCompSize += int64(compressedSize)
	typeStats.TotalTime += processingTime

	if typeStats.TotalOrigSize > 0 {
		typeStats.AverageRatio = float64(typeStats.TotalCompSize) / float64(typeStats.TotalOrigSize)
	}

	mc.stats.TypeStats[compressionType] = typeStats
}

// ConfigurePredefinedSettings sets up predefined compression settings for common content types
func (mc *MessageCompressor) ConfigurePredefinedSettings() {
	mc.config.ContentTypeSettings = map[string]CompressionSettings{
		"application/json": {
			Type:      CompressionGzip,
			Level:     6,
			Threshold: 512, // 512 bytes
			Enabled:   true,
		},
		"application/json+tool-result": {
			Type:      CompressionGzip,
			Level:     8, // Higher compression for tool results
			Threshold: 1024,
			Enabled:   true,
		},
		"application/json+resource-variant": {
			Type:      CompressionZlib,
			Level:     7,
			Threshold: 256,
			Enabled:   true,
		},
		"application/json+resource-evidence": {
			Type:      CompressionGzip,
			Level:     9, // Maximum compression for evidence data
			Threshold: 2048,
			Enabled:   true,
		},
		"application/json+batch-response": {
			Type:      CompressionGzip,
			Level:     8,
			Threshold: 4096, // Larger threshold for batch responses
			Enabled:   true,
		},
		"text/plain": {
			Type:      CompressionGzip,
			Level:     6,
			Threshold: 256,
			Enabled:   true,
		},
	}
}

// IsHealthy checks if the compressor is functioning properly
func (mc *MessageCompressor) IsHealthy() bool {
	// Test compression/decompression with sample data
	testData := []byte("This is a test message for health check compression and decompression functionality.")
	
	compressed, err := mc.CompressMessage(testData, "text/plain")
	if err != nil {
		return false
	}

	decompressed, err := mc.DecompressMessage(compressed)
	if err != nil {
		return false
	}

	return bytes.Equal(testData, decompressed)
}