package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/acmg-amp-mcp-server/internal/database"
	"github.com/acmg-amp-mcp-server/internal/domain"
)

// generateTestPassword creates a secure random password for test databases
func generateTestPassword() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a default test password if random generation fails
		return "test_fallback_password_123"
	}
	return "test_" + hex.EncodeToString(bytes)
}

func setupTestDB(t *testing.T) (*database.DB, func()) {
	ctx := context.Background()

	// Generate secure random password for test database
	testPassword := generateTestPassword()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword(testPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// Get connection details
	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	// Create database connection
	config := database.Config{
		Host:        host,
		Port:        port.Int(),
		Database:    "testdb",
		Username:    "testuser",
		Password:    testPassword,
		MaxConns:    10,
		MinConns:    2,
		MaxConnLife: time.Hour,
		MaxConnIdle: time.Minute * 30,
		SSLMode:     "disable",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	db, err := database.NewConnection(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
	}

	// Run migrations
	databaseURL := "postgres://testuser:" + testPassword + "@" + host + ":" + port.Port() + "/testdb?sslmode=disable"
	migrationRunner, err := database.NewMigrationRunner(databaseURL, "../../migrations", logger)
	if err != nil {
		t.Fatalf("Failed to create migration runner: %v", err)
	}

	if err := migrationRunner.Up(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	cleanup := func() {
		migrationRunner.Close()
		db.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}

	return db, cleanup
}

func TestVariantRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	repo := NewVariantRepository(db.Pool, logger)

	variant := &domain.StandardizedVariant{
		ID:           uuid.New().String(),
		HGVSGenomic:  "NC_000017.11:g.43094692G>A",
		Chromosome:   "17",
		Position:     43094692,
		Reference:    "G",
		Alternative:  "A",
		GeneSymbol:   "BRCA1",
		TranscriptID: "NM_007294.4",
		VariantType:  domain.GERMLINE,
	}

	ctx := context.Background()
	err := repo.Create(ctx, variant)
	if err != nil {
		t.Fatalf("Failed to create variant: %v", err)
	}

	// Verify the variant was created
	retrievedVariant, err := repo.GetByHGVS(ctx, variant.HGVSGenomic)
	if err != nil {
		t.Fatalf("Failed to retrieve variant: %v", err)
	}

	if retrievedVariant.ID != variant.ID {
		t.Errorf("Expected ID %s, got %s", variant.ID, retrievedVariant.ID)
	}

	if retrievedVariant.GeneSymbol != variant.GeneSymbol {
		t.Errorf("Expected gene symbol %s, got %s", variant.GeneSymbol, retrievedVariant.GeneSymbol)
	}
}

func TestVariantRepository_GetByGene(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	repo := NewVariantRepository(db.Pool, logger)

	// Create test variants
	variants := []*domain.StandardizedVariant{
		{
			ID:          uuid.New().String(),
			HGVSGenomic: "NC_000017.11:g.43094692G>A",
			Chromosome:  "17",
			Position:    43094692,
			Reference:   "G",
			Alternative: "A",
			GeneSymbol:  "BRCA1",
			VariantType: domain.GERMLINE,
		},
		{
			ID:          uuid.New().String(),
			HGVSGenomic: "NC_000017.11:g.43094693C>T",
			Chromosome:  "17",
			Position:    43094693,
			Reference:   "C",
			Alternative: "T",
			GeneSymbol:  "BRCA1",
			VariantType: domain.GERMLINE,
		},
	}

	ctx := context.Background()
	for _, variant := range variants {
		if err := repo.Create(ctx, variant); err != nil {
			t.Fatalf("Failed to create variant: %v", err)
		}
	}

	// Test GetByGene
	retrievedVariants, err := repo.GetByGene(ctx, "BRCA1", 10, 0)
	if err != nil {
		t.Fatalf("Failed to get variants by gene: %v", err)
	}

	if len(retrievedVariants) != 2 {
		t.Errorf("Expected 2 variants, got %d", len(retrievedVariants))
	}

	// Verify all retrieved variants have the correct gene symbol
	for _, variant := range retrievedVariants {
		if variant.GeneSymbol != "BRCA1" {
			t.Errorf("Expected gene symbol BRCA1, got %s", variant.GeneSymbol)
		}
	}
}

func TestVariantRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	repo := NewVariantRepository(db.Pool, logger)

	variant := &domain.StandardizedVariant{
		ID:          uuid.New().String(),
		HGVSGenomic: "NC_000017.11:g.43094692G>A",
		Chromosome:  "17",
		Position:    43094692,
		Reference:   "G",
		Alternative: "A",
		GeneSymbol:  "BRCA1",
		VariantType: domain.GERMLINE,
	}

	ctx := context.Background()
	if err := repo.Create(ctx, variant); err != nil {
		t.Fatalf("Failed to create variant: %v", err)
	}

	// Update the variant
	variant.TranscriptID = "NM_007294.4"
	if err := repo.Update(ctx, variant); err != nil {
		t.Fatalf("Failed to update variant: %v", err)
	}

	// Verify the update
	updatedVariant, err := repo.GetByHGVS(ctx, variant.HGVSGenomic)
	if err != nil {
		t.Fatalf("Failed to retrieve updated variant: %v", err)
	}

	if updatedVariant.TranscriptID != "NM_007294.4" {
		t.Errorf("Expected transcript ID NM_007294.4, got %s", updatedVariant.TranscriptID)
	}
}

func TestVariantRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	repo := NewVariantRepository(db.Pool, logger)

	variant := &domain.StandardizedVariant{
		ID:          uuid.New().String(),
		HGVSGenomic: "NC_000017.11:g.43094692G>A",
		Chromosome:  "17",
		Position:    43094692,
		Reference:   "G",
		Alternative: "A",
		GeneSymbol:  "BRCA1",
		VariantType: domain.GERMLINE,
	}

	ctx := context.Background()
	if err := repo.Create(ctx, variant); err != nil {
		t.Fatalf("Failed to create variant: %v", err)
	}

	// Delete the variant
	variantUUID, err := uuid.Parse(variant.ID)
	if err != nil {
		t.Fatalf("Failed to parse variant ID: %v", err)
	}

	if err := repo.Delete(ctx, variantUUID); err != nil {
		t.Fatalf("Failed to delete variant: %v", err)
	}

	// Verify the variant was deleted
	_, err = repo.GetByHGVS(ctx, variant.HGVSGenomic)
	if err == nil {
		t.Error("Expected error when getting deleted variant, got nil")
	}
}
