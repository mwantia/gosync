package models

import (
	"time"

	"gorm.io/gorm"
)

// File represents metadata for a file stored in a backend
type File struct {
	ID         uint   `gorm:"primaryKey"`
	BackendID  string `gorm:"type:text;not null;index:idx_backend_path"`
	Path       string `gorm:"type:text;not null;index:idx_backend_path"`

	// File metadata
	Size       int64  `gorm:"not null"`
	MD5Hash    string `gorm:"type:text"`
	SHA256Hash string `gorm:"type:text"`
	ETag       string `gorm:"type:text"`

	// Timestamps
	ModifiedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`

	// Relationships
	Backend Backend `gorm:"foreignKey:BackendID;references:ID"`
	Tags    []Tag   `gorm:"foreignKey:FileID;constraint:OnDelete:CASCADE"`
}
