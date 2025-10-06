package store

import (
	"context"

	"github.com/mwantia/gosync/pkg/db/models"
)

// MetadataStore defines the interface for database operations
type MetadataStore interface {
	// Lifecycle
	Connect(ctx context.Context) error
	Close() error
	Migrate(ctx context.Context) error
	Health(ctx context.Context) error

	// Backend operations
	CreateBackend(ctx context.Context, backend *models.Backend) error
	GetBackend(ctx context.Context, id string) (*models.Backend, error)
	ListBackends(ctx context.Context) ([]models.Backend, error)
	UpdateBackend(ctx context.Context, backend *models.Backend) error
	DeleteBackend(ctx context.Context, id string) error

	// File operations
	CreateFile(ctx context.Context, file *models.File) error
	GetFile(ctx context.Context, backendID, path string) (*models.File, error)
	ListFiles(ctx context.Context, backendID, pathPrefix string, limit, offset int) ([]models.File, error)
	UpdateFile(ctx context.Context, file *models.File) error
	DeleteFile(ctx context.Context, id uint) error
	DeleteFilesByBackend(ctx context.Context, backendID string) error

	// Tag operations
	CreateTag(ctx context.Context, tag *models.Tag) error
	GetFileTags(ctx context.Context, fileID uint) ([]models.Tag, error)
	GetFilesByTag(ctx context.Context, key, value string, limit, offset int) ([]models.File, error)
	DeleteTag(ctx context.Context, id uint) error
	DeleteFileTags(ctx context.Context, fileID uint) error

	// Filter operations
	CreateFilter(ctx context.Context, filter *models.Filter) error
	GetFilter(ctx context.Context, virtualPath string) (*models.Filter, error)
	ListFilters(ctx context.Context) ([]models.Filter, error)
	UpdateFilter(ctx context.Context, filter *models.Filter) error
	DeleteFilter(ctx context.Context, id uint) error

	// Sync operations
	CreateSyncConfig(ctx context.Context, config *models.SyncConfig) error
	GetSyncConfig(ctx context.Context, name string) (*models.SyncConfig, error)
	ListSyncConfigs(ctx context.Context) ([]models.SyncConfig, error)
	UpdateSyncConfig(ctx context.Context, config *models.SyncConfig) error
	DeleteSyncConfig(ctx context.Context, id uint) error

	// Sync state operations
	CreateSyncState(ctx context.Context, state *models.SyncState) error
	GetSyncState(ctx context.Context, syncConfigID uint, backendID, clientID string) (*models.SyncState, error)
	UpdateSyncState(ctx context.Context, state *models.SyncState) error
	DeleteSyncState(ctx context.Context, id uint) error
}
