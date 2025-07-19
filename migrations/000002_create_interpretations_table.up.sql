-- Create interpretations table for storing variant classification results
CREATE TABLE IF NOT EXISTS interpretations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id UUID NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    classification VARCHAR(20) NOT NULL CHECK (classification IN ('PATHOGENIC', 'LIKELY_PATHOGENIC', 'VUS', 'LIKELY_BENIGN', 'BENIGN')),
    confidence_level VARCHAR(20) NOT NULL CHECK (confidence_level IN ('HIGH', 'MEDIUM', 'LOW')),
    applied_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    report_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    processing_time_ms INTEGER NOT NULL CHECK (processing_time_ms >= 0),
    client_id VARCHAR(100),
    request_id VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT interpretations_processing_time_reasonable CHECK (processing_time_ms < 300000) -- Max 5 minutes
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_interpretations_variant_id ON interpretations (variant_id);
CREATE INDEX IF NOT EXISTS idx_interpretations_classification ON interpretations (classification);
CREATE INDEX IF NOT EXISTS idx_interpretations_confidence_level ON interpretations (confidence_level);
CREATE INDEX IF NOT EXISTS idx_interpretations_client_id ON interpretations (client_id);
CREATE INDEX IF NOT EXISTS idx_interpretations_created_at ON interpretations (created_at);
CREATE INDEX IF NOT EXISTS idx_interpretations_processing_time ON interpretations (processing_time_ms);

-- Create GIN indexes for JSONB columns for efficient querying
CREATE INDEX IF NOT EXISTS idx_interpretations_applied_rules_gin ON interpretations USING GIN (applied_rules);
CREATE INDEX IF NOT EXISTS idx_interpretations_evidence_summary_gin ON interpretations USING GIN (evidence_summary);

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_interpretations_updated_at 
    BEFORE UPDATE ON interpretations 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();