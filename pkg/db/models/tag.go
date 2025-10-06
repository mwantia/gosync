package models

import (
	"time"

	"gorm.io/gorm"
)

// Tag represents a key-value tag attached to a file
type Tag struct {
	ID     uint   `gorm:"primaryKey"`
	FileID uint   `gorm:"not null;index:idx_file_tags"`
	Key    string `gorm:"type:text;not null;index:idx_tag_key"`
	Value  string `gorm:"type:text;not null;index:idx_tag_value"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Relationships
	File File `gorm:"foreignKey:FileID;references:ID"`
}
