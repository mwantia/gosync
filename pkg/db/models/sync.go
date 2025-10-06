package models

import (
	"time"

	"gorm.io/gorm"
)

// SyncConfig represents a sync mirror configuration between source and destination
type SyncConfig struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"type:text;not null;uniqueIndex"`
	SourcePath  string `gorm:"type:text;not null"` // Virtual path (backend/path or filter/path)
	DestPath    string `gorm:"type:text;not null"` // Local or virtual path
	Direction   string `gorm:"type:text;not null"` // "bidirectional", "download", "upload"
	Enabled     bool   `gorm:"default:true"`

	// Sync settings
	Interval      int64  `gorm:"not null"` // Seconds between syncs
	Workers       int    `gorm:"default:4"`
	ChunkSize     int64  `gorm:"default:5242880"` // 5MB default
	IgnorePattern string `gorm:"type:text"` // Glob pattern for ignoring files

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Relationships
	States []SyncState `gorm:"foreignKey:SyncConfigID;constraint:OnDelete:CASCADE"`
}

// SyncState tracks per-client, per-backend sync cursor for resumability
type SyncState struct {
	ID           uint   `gorm:"primaryKey"`
	SyncConfigID uint   `gorm:"not null;index:idx_sync_backend"`
	BackendID    string `gorm:"type:text;not null;index:idx_sync_backend"`
	ClientID     string `gorm:"type:text;not null"` // Identifier for this client instance

	// State tracking
	LastSyncAt    time.Time
	LastCursor    string `gorm:"type:text"` // Path or token for resuming sync
	FilesScanned  int64  `gorm:"default:0"`
	FilesSynced   int64  `gorm:"default:0"`
	BytesSynced   int64  `gorm:"default:0"`
	ErrorCount    int    `gorm:"default:0"`
	LastError     string `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time

	// Relationships
	SyncConfig SyncConfig `gorm:"foreignKey:SyncConfigID;references:ID"`
}
