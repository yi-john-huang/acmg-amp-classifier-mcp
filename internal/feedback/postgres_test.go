package feedback

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestDB returns a database connection for testing.
// Skip test if DATABASE_URL is not set.
func getTestDB(t *testing.T) *sql.DB {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping PostgreSQL tests")
	}

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)

	// Create feedback table for testing
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS feedback (
			id BIGSERIAL PRIMARY KEY,
			variant TEXT NOT NULL,
			normalized_hgvs TEXT NOT NULL,
			cancer_type TEXT DEFAULT '',
			suggested_classification TEXT NOT NULL,
			user_classification TEXT NOT NULL,
			user_agreed BOOLEAN NOT NULL DEFAULT FALSE,
			evidence_summary TEXT DEFAULT '',
			notes TEXT DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			CONSTRAINT feedback_normalized_hgvs_cancer_type_unique UNIQUE (normalized_hgvs, cancer_type)
		)
	`)
	require.NoError(t, err)

	// Clean up before test
	_, err = db.Exec("DELETE FROM feedback")
	require.NoError(t, err)

	return db
}

func TestPostgresStore_Save(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	ctx := context.Background()
	fb := &Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.3:c.5266dupC",
		CancerType:              "breast",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
		EvidenceSummary:         "Strong evidence for pathogenicity",
		Notes:                   "User confirmed classification",
	}

	err = store.Save(ctx, fb)
	require.NoError(t, err)
	assert.NotZero(t, fb.ID)
	assert.NotZero(t, fb.CreatedAt)
	assert.NotZero(t, fb.UpdatedAt)
}

func TestPostgresStore_SaveUpdate(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	ctx := context.Background()
	fb := &Feedback{
		Variant:                 "BRCA1:c.5266dupC",
		NormalizedHGVS:          "NM_007294.3:c.5266dupC",
		CancerType:              "breast",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationVUS,
		UserAgreed:              false,
	}

	// First save
	err = store.Save(ctx, fb)
	require.NoError(t, err)
	originalID := fb.ID

	// Update
	fb.UserClassification = ClassificationPathogenic
	fb.UserAgreed = true
	fb.Notes = "Updated after review"

	err = store.Save(ctx, fb)
	require.NoError(t, err)

	// Should have same ID (upsert)
	assert.Equal(t, originalID, fb.ID)

	// Verify update
	retrieved, err := store.Get(ctx, fb.NormalizedHGVS, fb.CancerType)
	require.NoError(t, err)
	assert.Equal(t, ClassificationPathogenic, retrieved.UserClassification)
	assert.True(t, retrieved.UserAgreed)
	assert.Equal(t, "Updated after review", retrieved.Notes)
}

func TestPostgresStore_Get(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test not found
	fb, err := store.Get(ctx, "nonexistent", "")
	require.NoError(t, err)
	assert.Nil(t, fb)

	// Save and retrieve
	saved := &Feedback{
		Variant:                 "TP53:p.R273H",
		NormalizedHGVS:          "NM_000546.5:c.818G>A",
		CancerType:              "",
		SuggestedClassification: ClassificationPathogenic,
		UserClassification:      ClassificationPathogenic,
		UserAgreed:              true,
	}
	err = store.Save(ctx, saved)
	require.NoError(t, err)

	retrieved, err := store.Get(ctx, saved.NormalizedHGVS, saved.CancerType)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, saved.Variant, retrieved.Variant)
	assert.Equal(t, saved.NormalizedHGVS, retrieved.NormalizedHGVS)
	assert.Equal(t, saved.SuggestedClassification, retrieved.SuggestedClassification)
}

func TestPostgresStore_List(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	ctx := context.Background()

	// Insert multiple entries
	for i := 0; i < 5; i++ {
		fb := &Feedback{
			Variant:                 "test",
			NormalizedHGVS:          "NM_000001.1:c." + string(rune('A'+i)) + ">G",
			CancerType:              "",
			SuggestedClassification: ClassificationVUS,
			UserClassification:      ClassificationVUS,
			UserAgreed:              true,
		}
		err = store.Save(ctx, fb)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Test pagination
	list, err := store.List(ctx, 3, 0)
	require.NoError(t, err)
	assert.Len(t, list, 3)

	list, err = store.List(ctx, 3, 3)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestPostgresStore_Count(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	ctx := context.Background()

	// Initially empty
	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add entries
	for i := 0; i < 3; i++ {
		fb := &Feedback{
			Variant:                 "test",
			NormalizedHGVS:          "NM_" + string(rune('0'+i)) + ":c.1A>G",
			SuggestedClassification: ClassificationVUS,
			UserClassification:      ClassificationVUS,
		}
		err = store.Save(ctx, fb)
		require.NoError(t, err)
	}

	count, err = store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestPostgresStore_Delete(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	ctx := context.Background()

	// Save entry
	fb := &Feedback{
		Variant:                 "test",
		NormalizedHGVS:          "NM_000001.1:c.1A>G",
		SuggestedClassification: ClassificationVUS,
		UserClassification:      ClassificationVUS,
	}
	err = store.Save(ctx, fb)
	require.NoError(t, err)

	// Delete
	err = store.Delete(ctx, fb.ID)
	require.NoError(t, err)

	// Verify deleted
	retrieved, err := store.Get(ctx, fb.NormalizedHGVS, fb.CancerType)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}
