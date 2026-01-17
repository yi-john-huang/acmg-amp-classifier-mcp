package feedback

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteStore(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "feedback-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Act
	store, err := NewSQLiteStore(dbPath)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")
}

func TestSQLiteStore_Save(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	feedback := &Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.4:c.5266dup",
		CancerType:              "breast",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationLikelyPathogenic,
		UserAgreed:              false,
		EvidenceSummary:         "ClinVar: Pathogenic, gnomAD: absent",
		Notes:                   "Additional family history considered",
	}

	// Act
	err := store.Save(ctx, feedback)

	// Assert
	require.NoError(t, err)
	assert.NotZero(t, feedback.ID, "ID should be assigned")
	assert.False(t, feedback.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, feedback.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestSQLiteStore_Save_Update(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save initial feedback
	feedback := &Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.4:c.5266dup",
		CancerType:              "breast",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
	}
	err := store.Save(ctx, feedback)
	require.NoError(t, err)
	originalID := feedback.ID

	// Update with same variant + cancer_type
	feedback.UserClassification = ClassificationLikelyPathogenic
	feedback.UserAgreed = false
	feedback.Notes = "Updated after review"

	err = store.Save(ctx, feedback)
	require.NoError(t, err)

	// Assert - should update, not create new
	assert.Equal(t, originalID, feedback.ID, "Should update existing record")

	// Verify update
	retrieved, err := store.Get(ctx, "NM_007294.4:c.5266dup", "breast")
	require.NoError(t, err)
	assert.Equal(t, ClassificationLikelyPathogenic, retrieved.UserClassification)
	assert.Equal(t, "Updated after review", retrieved.Notes)
}

func TestSQLiteStore_Get(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save feedback
	feedback := &Feedback{
		Variant:                 "CFTR:c.1521_1523del",
		NormalizedHGVS:          "NM_000492.3:c.1521_1523del",
		CancerType:              "",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
	}
	err := store.Save(ctx, feedback)
	require.NoError(t, err)

	// Act
	retrieved, err := store.Get(ctx, "NM_000492.3:c.1521_1523del", "")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, feedback.NormalizedHGVS, retrieved.NormalizedHGVS)
	assert.Equal(t, feedback.UserClassification, retrieved.UserClassification)
}

func TestSQLiteStore_Get_WithCancerType(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save same variant with different cancer types
	feedback1 := &Feedback{
		Variant:                 "TP53:p.R273H",
		NormalizedHGVS:          "NM_000546.6:c.817C>A",
		CancerType:              "lung",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
	}
	err := store.Save(ctx, feedback1)
	require.NoError(t, err)

	feedback2 := &Feedback{
		Variant:                 "TP53:p.R273H",
		NormalizedHGVS:          "NM_000546.6:c.817C>A",
		CancerType:              "breast",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationLikelyPathogenic,
		UserAgreed:              false,
	}
	err = store.Save(ctx, feedback2)
	require.NoError(t, err)

	// Act - get with specific cancer type
	lung, err := store.Get(ctx, "NM_000546.6:c.817C>A", "lung")
	require.NoError(t, err)
	assert.Equal(t, ClassificationPathogenic, lung.UserClassification)

	breast, err := store.Get(ctx, "NM_000546.6:c.817C>A", "breast")
	require.NoError(t, err)
	assert.Equal(t, ClassificationLikelyPathogenic, breast.UserClassification)
}

func TestSQLiteStore_Get_NotFound(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Act
	retrieved, err := store.Get(ctx, "NM_000000.0:c.1A>G", "")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, retrieved, "Should return nil for not found")
}

func TestSQLiteStore_List(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save multiple feedback entries
	variants := []string{
		"NM_000492.3:c.1521_1523del",
		"NM_007294.4:c.5266dup",
		"NM_000546.6:c.817C>A",
	}

	for i, v := range variants {
		feedback := &Feedback{
			Variant:                 v,
			NormalizedHGVS:          v,
			SuggestedClassification: ClassificationPathogenic,
			UserClassification:      ClassificationPathogenic,
			UserAgreed:              true,
		}
		err := store.Save(ctx, feedback)
		require.NoError(t, err, "Failed to save feedback %d", i)
	}

	// Act
	list, err := store.List(ctx, 10, 0)

	// Assert
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestSQLiteStore_List_Pagination(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save 5 entries
	for i := 0; i < 5; i++ {
		feedback := &Feedback{
			Variant:                 "variant" + string(rune('A'+i)),
			NormalizedHGVS:          "NM_00000" + string(rune('0'+i)) + ".1:c.1A>G",
			SuggestedClassification: ClassificationVUS,
			UserClassification:      ClassificationVUS,
			UserAgreed:              true,
		}
		err := store.Save(ctx, feedback)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Act - get first page
	page1, err := store.List(ctx, 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	// Act - get second page
	page2, err := store.List(ctx, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	// Act - get third page
	page3, err := store.List(ctx, 2, 4)
	require.NoError(t, err)
	assert.Len(t, page3, 1)
}

func TestSQLiteStore_Count(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save 3 entries
	for i := 0; i < 3; i++ {
		feedback := &Feedback{
			Variant:                 "variant" + string(rune('A'+i)),
			NormalizedHGVS:          "NM_00000" + string(rune('0'+i)) + ".1:c.1A>G",
			SuggestedClassification: ClassificationVUS,
			UserClassification:      ClassificationVUS,
			UserAgreed:              true,
		}
		err := store.Save(ctx, feedback)
		require.NoError(t, err)
	}

	// Act
	count, err := store.Count(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestSQLiteStore_Delete(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save feedback
	feedback := &Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.4:c.5266dup",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
	}
	err := store.Save(ctx, feedback)
	require.NoError(t, err)

	// Act
	err = store.Delete(ctx, feedback.ID)

	// Assert
	require.NoError(t, err)

	// Verify deletion
	retrieved, err := store.Get(ctx, "NM_007294.4:c.5266dup", "")
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestSQLiteStore_ExportJSON(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save feedback
	feedback := &Feedback{
		Variant:                 "CFTR:c.1521_1523del",
		NormalizedHGVS:          "NM_000492.3:c.1521_1523del",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
		Notes:                   "Well-characterized variant",
	}
	err := store.Save(ctx, feedback)
	require.NoError(t, err)

	// Act
	var buf bytes.Buffer
	err = store.ExportJSON(ctx, &buf)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "NM_000492.3:c.1521_1523del")
	assert.Contains(t, buf.String(), "Well-characterized variant")
	assert.Contains(t, buf.String(), `"version"`)
	assert.Contains(t, buf.String(), `"count"`)
}

func TestSQLiteStore_ImportJSON(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create JSON to import
	jsonData := `{
		"version": "1.0",
		"exported_at": "2026-01-17T10:00:00Z",
		"count": 2,
		"feedback": [
			{
				"variant": "BRCA1:c.5266dupC",
				"normalized_hgvs": "NM_007294.4:c.5266dup",
				"cancer_type": "breast",
				"suggested_classification": "Pathogenic",
				"user_classification": "Pathogenic",
				"user_agreed": true
			},
			{
				"variant": "TP53:p.R273H",
				"normalized_hgvs": "NM_000546.6:c.817C>A",
				"cancer_type": "lung",
				"suggested_classification": "Pathogenic",
				"user_classification": "Likely Pathogenic",
				"user_agreed": false,
				"notes": "Additional evidence needed"
			}
		]
	}`

	// Act
	imported, skipped, err := store.ImportJSON(ctx, bytes.NewReader([]byte(jsonData)))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, imported)
	assert.Equal(t, 0, skipped)

	// Verify imports
	count, _ := store.Count(ctx)
	assert.Equal(t, int64(2), count)

	brca1, err := store.Get(ctx, "NM_007294.4:c.5266dup", "breast")
	require.NoError(t, err)
	assert.Equal(t, ClassificationPathogenic, brca1.UserClassification)

	tp53, err := store.Get(ctx, "NM_000546.6:c.817C>A", "lung")
	require.NoError(t, err)
	assert.Equal(t, ClassificationLikelyPathogenic, tp53.UserClassification)
	assert.Equal(t, "Additional evidence needed", tp53.Notes)
}

func TestSQLiteStore_ImportJSON_SkipDuplicates(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Save existing feedback
	existing := &Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.4:c.5266dup",
		CancerType:              "breast",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
	}
	err := store.Save(ctx, existing)
	require.NoError(t, err)

	// Import with duplicate
	jsonData := `{
		"version": "1.0",
		"count": 2,
		"feedback": [
			{
				"normalized_hgvs": "NM_007294.4:c.5266dup",
				"cancer_type": "breast",
				"suggested_classification": "Pathogenic",
				"user_classification": "VUS",
				"user_agreed": false
			},
			{
				"normalized_hgvs": "NM_000546.6:c.817C>A",
				"suggested_classification": "Pathogenic",
				"user_classification": "Pathogenic",
				"user_agreed": true
			}
		]
	}`

	// Act
	imported, skipped, err := store.ImportJSON(ctx, bytes.NewReader([]byte(jsonData)))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, imported)
	assert.Equal(t, 1, skipped)

	// Verify existing wasn't overwritten
	brca1, _ := store.Get(ctx, "NM_007294.4:c.5266dup", "breast")
	assert.Equal(t, ClassificationPathogenic, brca1.UserClassification, "Existing should not be overwritten")
}

// Helper function to create a test store
func createTestStore(t *testing.T) *SQLiteStore {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "feedback-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)

	return store
}
