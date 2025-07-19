-- Create variants table for storing genetic variant information
CREATE TABLE IF NOT EXISTS variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hgvs_notation VARCHAR(255) NOT NULL,
    chromosome VARCHAR(10) NOT NULL,
    position BIGINT NOT NULL,
    reference VARCHAR(1000) NOT NULL,
    alternative VARCHAR(1000) NOT NULL,
    gene_symbol VARCHAR(50),
    transcript_id VARCHAR(50),
    variant_type VARCHAR(20) NOT NULL CHECK (variant_type IN ('GERMLINE', 'SOMATIC')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT variants_hgvs_unique UNIQUE (hgvs_notation),
    CONSTRAINT variants_position_check CHECK (position > 0),
    CONSTRAINT variants_ref_alt_check CHECK (reference != '' AND alternative != '')
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_variants_chromosome_position ON variants (chromosome, position);
CREATE INDEX IF NOT EXISTS idx_variants_gene_symbol ON variants (gene_symbol);
CREATE INDEX IF NOT EXISTS idx_variants_variant_type ON variants (variant_type);
CREATE INDEX IF NOT EXISTS idx_variants_created_at ON variants (created_at);

-- Create function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_variants_updated_at 
    BEFORE UPDATE ON variants 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();