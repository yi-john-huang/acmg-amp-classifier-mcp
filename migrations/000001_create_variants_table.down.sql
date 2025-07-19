-- Drop trigger and function
DROP TRIGGER IF EXISTS update_variants_updated_at ON variants;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_variants_created_at;
DROP INDEX IF EXISTS idx_variants_variant_type;
DROP INDEX IF EXISTS idx_variants_gene_symbol;
DROP INDEX IF EXISTS idx_variants_chromosome_position;

-- Drop table
DROP TABLE IF EXISTS variants;