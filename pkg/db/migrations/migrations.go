package migrations

import (
	"context"
	"fmt"

	"github.com/mwantia/gosync/pkg/db/models"
	"gorm.io/gorm"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          func(*gorm.DB) error
	Down        func(*gorm.DB) error
}

// migrationHistory tracks applied migrations
type migrationHistory struct {
	ID          uint   `gorm:"primaryKey"`
	Version     int    `gorm:"uniqueIndex;not null"`
	Description string `gorm:"type:text"`
	AppliedAt   int64  `gorm:"autoCreateTime"`
}

// Migrator handles database migrations
type Migrator struct {
	db         *gorm.DB
	migrations []Migration
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{
		db:         db,
		migrations: allMigrations(),
	}
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate(ctx context.Context) error {
	// Ensure migration history table exists
	if err := m.db.WithContext(ctx).AutoMigrate(&migrationHistory{}); err != nil {
		return fmt.Errorf("failed to create migration history table: %w", err)
	}

	// Get applied migrations
	var applied []migrationHistory
	if err := m.db.WithContext(ctx).Find(&applied).Error; err != nil {
		return fmt.Errorf("failed to query migration history: %w", err)
	}

	appliedVersions := make(map[int]bool)
	for _, a := range applied {
		appliedVersions[a.Version] = true
	}

	// Run pending migrations
	for _, migration := range m.migrations {
		if appliedVersions[migration.Version] {
			continue
		}

		if err := m.runMigration(ctx, migration); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Description, err)
		}
	}

	return nil
}

// Rollback rolls back the last applied migration
func (m *Migrator) Rollback(ctx context.Context) error {
	// Get last applied migration
	var last migrationHistory
	if err := m.db.WithContext(ctx).Order("version DESC").First(&last).Error; err != nil {
		return fmt.Errorf("no migrations to rollback: %w", err)
	}

	// Find migration
	var migration *Migration
	for _, m := range m.migrations {
		if m.Version == last.Version {
			migration = &m
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %d not found", last.Version)
	}

	// Run down migration
	if err := migration.Down(m.db.WithContext(ctx)); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Remove from history
	if err := m.db.WithContext(ctx).Delete(&last).Error; err != nil {
		return fmt.Errorf("failed to update migration history: %w", err)
	}

	return nil
}

// Status returns migration status
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	var applied []migrationHistory
	if err := m.db.WithContext(ctx).Find(&applied).Error; err != nil {
		return nil, fmt.Errorf("failed to query migration history: %w", err)
	}

	appliedVersions := make(map[int]bool)
	for _, a := range applied {
		appliedVersions[a.Version] = true
	}

	var statuses []MigrationStatus
	for _, migration := range m.migrations {
		statuses = append(statuses, MigrationStatus{
			Version:     migration.Version,
			Description: migration.Description,
			Applied:     appliedVersions[migration.Version],
		})
	}

	return statuses, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int
	Description string
	Applied     bool
}

func (m *Migrator) runMigration(ctx context.Context, migration Migration) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Run migration
		if err := migration.Up(tx); err != nil {
			return err
		}

		// Record in history
		history := migrationHistory{
			Version:     migration.Version,
			Description: migration.Description,
		}
		return tx.Create(&history).Error
	})
}

// allMigrations returns all migrations in order
func allMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Initial schema creation",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(
					&models.Backend{},
					&models.File{},
					&models.Tag{},
					&models.Filter{},
					&models.SyncConfig{},
					&models.SyncState{},
				)
			},
			Down: func(db *gorm.DB) error {
				return db.Migrator().DropTable(
					&models.SyncState{},
					&models.SyncConfig{},
					&models.Filter{},
					&models.Tag{},
					&models.File{},
					&models.Backend{},
				)
			},
		},
	}
}
