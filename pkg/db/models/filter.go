package models

import (
	"time"

	"gorm.io/gorm"
)

// Filter represents a dynamic virtual path based on tag queries
type Filter struct {
	ID              uint   `gorm:"primaryKey"`
	VirtualPath     string `gorm:"type:text;not null;uniqueIndex"`
	Name            string `gorm:"type:text;not null"`
	QueryExpression string `gorm:"type:text;not null"` // e.g., "tag:colour=red AND tag:event=vacation"
	Description     string `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
