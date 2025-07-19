-- Drop trigger
DROP TRIGGER IF EXISTS update_interpretations_updated_at ON interpretations;

-- Drop indexes
DROP INDEX IF EXISTS idx_interpretations_evidence_summary_gin;
DROP INDEX IF EXISTS idx_interpretations_applied_rules_gin;
DROP INDEX IF EXISTS idx_interpretations_processing_time;
DROP INDEX IF EXISTS idx_interpretations_created_at;
DROP INDEX IF EXISTS idx_interpretations_client_id;
DROP INDEX IF EXISTS idx_interpretations_confidence_level;
DROP INDEX IF EXISTS idx_interpretations_classification;
DROP INDEX IF EXISTS idx_interpretations_variant_id;

-- Drop table
DROP TABLE IF EXISTS interpretations;