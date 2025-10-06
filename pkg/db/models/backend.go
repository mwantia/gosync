package models

import (
	"time"

	"gorm.io/gorm"
)

// Backend represents an S3-compatible storage backend configuration
type Backend struct {
	ID        string `gorm:"primaryKey;type:text"`
	Name      string `gorm:"type:text;not null"`
	Endpoint  string `gorm:"type:text;not null"`
	Region    string `gorm:"type:text"`
	Bucket    string `gorm:"type:text;not null"`
	UseSSL    bool   `gorm:"default:true"`

	// Encrypted credentials
	AccessKey string `gorm:"type:text;not null"`
	SecretKey string `gorm:"type:text;not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Relationships
	Files []File `gorm:"foreignKey:BackendID;constraint:OnDelete:CASCADE"`
}
