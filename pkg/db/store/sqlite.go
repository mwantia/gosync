package store

import (
	"context"
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/mwantia/gosync/pkg/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQLiteStore implements MetadataStore using SQLite
type SQLiteStore struct {
	db   *gorm.DB
	path string
}

// DB returns the underlying GORM database instance
func (s *SQLiteStore) DB() *gorm.DB {
	return s.db
}

// SQLiteConfig holds SQLite-specific configuration
type SQLiteConfig struct {
	Path         string
	MaxOpenConns int
	LogLevel     logger.LogLevel
}

// NewSQLiteStore creates a new SQLite-backed metadata store
func NewSQLiteStore(cfg SQLiteConfig) (*SQLiteStore, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	// Default to silent logging
	if cfg.LogLevel == 0 {
		cfg.LogLevel = logger.Silent
	}

	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	return &SQLiteStore{
		db:   db,
		path: cfg.Path,
	}, nil
}

// Connect initializes the database connection
func (s *SQLiteStore) Connect(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(1) // SQLite only supports 1 writer
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return sqlDB.PingContext(ctx)
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	return sqlDB.Close()
}

// Migrate runs database migrations
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(
		&models.Backend{},
		&models.File{},
		&models.Tag{},
		&models.Filter{},
		&models.SyncConfig{},
		&models.SyncState{},
	)
}

// Health checks database connectivity
func (s *SQLiteStore) Health(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// Backend operations

func (s *SQLiteStore) CreateBackend(ctx context.Context, backend *models.Backend) error {
	return s.db.WithContext(ctx).Create(backend).Error
}

func (s *SQLiteStore) GetBackend(ctx context.Context, id string) (*models.Backend, error) {
	var backend models.Backend
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&backend).Error
	if err != nil {
		return nil, err
	}
	return &backend, nil
}

func (s *SQLiteStore) ListBackends(ctx context.Context) ([]models.Backend, error) {
	var backends []models.Backend
	err := s.db.WithContext(ctx).Find(&backends).Error
	return backends, err
}

func (s *SQLiteStore) UpdateBackend(ctx context.Context, backend *models.Backend) error {
	return s.db.WithContext(ctx).Save(backend).Error
}

func (s *SQLiteStore) DeleteBackend(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Backend{}, "id = ?", id).Error
}

// File operations

func (s *SQLiteStore) CreateFile(ctx context.Context, file *models.File) error {
	return s.db.WithContext(ctx).Create(file).Error
}

func (s *SQLiteStore) GetFile(ctx context.Context, backendID, path string) (*models.File, error) {
	var file models.File
	err := s.db.WithContext(ctx).
		Where("backend_id = ? AND path = ?", backendID, path).
		First(&file).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (s *SQLiteStore) ListFiles(ctx context.Context, backendID, pathPrefix string, limit, offset int) ([]models.File, error) {
	var files []models.File
	query := s.db.WithContext(ctx).Where("backend_id = ?", backendID)

	if pathPrefix != "" {
		query = query.Where("path LIKE ?", pathPrefix+"%")
	}

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&files).Error
	return files, err
}

func (s *SQLiteStore) UpdateFile(ctx context.Context, file *models.File) error {
	return s.db.WithContext(ctx).Save(file).Error
}

func (s *SQLiteStore) DeleteFile(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.File{}, id).Error
}

func (s *SQLiteStore) DeleteFilesByBackend(ctx context.Context, backendID string) error {
	return s.db.WithContext(ctx).Where("backend_id = ?", backendID).Delete(&models.File{}).Error
}

// Tag operations

func (s *SQLiteStore) CreateTag(ctx context.Context, tag *models.Tag) error {
	return s.db.WithContext(ctx).Create(tag).Error
}

func (s *SQLiteStore) GetFileTags(ctx context.Context, fileID uint) ([]models.Tag, error) {
	var tags []models.Tag
	err := s.db.WithContext(ctx).Where("file_id = ?", fileID).Find(&tags).Error
	return tags, err
}

func (s *SQLiteStore) GetFilesByTag(ctx context.Context, key, value string, limit, offset int) ([]models.File, error) {
	var files []models.File
	query := s.db.WithContext(ctx).
		Joins("JOIN tags ON tags.file_id = files.id").
		Where("tags.key = ? AND tags.value = ?", key, value)

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&files).Error
	return files, err
}

func (s *SQLiteStore) DeleteTag(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.Tag{}, id).Error
}

func (s *SQLiteStore) DeleteFileTags(ctx context.Context, fileID uint) error {
	return s.db.WithContext(ctx).Where("file_id = ?", fileID).Delete(&models.Tag{}).Error
}

// Filter operations

func (s *SQLiteStore) CreateFilter(ctx context.Context, filter *models.Filter) error {
	return s.db.WithContext(ctx).Create(filter).Error
}

func (s *SQLiteStore) GetFilter(ctx context.Context, virtualPath string) (*models.Filter, error) {
	var filter models.Filter
	err := s.db.WithContext(ctx).Where("virtual_path = ?", virtualPath).First(&filter).Error
	if err != nil {
		return nil, err
	}
	return &filter, nil
}

func (s *SQLiteStore) ListFilters(ctx context.Context) ([]models.Filter, error) {
	var filters []models.Filter
	err := s.db.WithContext(ctx).Find(&filters).Error
	return filters, err
}

func (s *SQLiteStore) UpdateFilter(ctx context.Context, filter *models.Filter) error {
	return s.db.WithContext(ctx).Save(filter).Error
}

func (s *SQLiteStore) DeleteFilter(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.Filter{}, id).Error
}

// Sync operations

func (s *SQLiteStore) CreateSyncConfig(ctx context.Context, config *models.SyncConfig) error {
	return s.db.WithContext(ctx).Create(config).Error
}

func (s *SQLiteStore) GetSyncConfig(ctx context.Context, name string) (*models.SyncConfig, error) {
	var config models.SyncConfig
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *SQLiteStore) ListSyncConfigs(ctx context.Context) ([]models.SyncConfig, error) {
	var configs []models.SyncConfig
	err := s.db.WithContext(ctx).Find(&configs).Error
	return configs, err
}

func (s *SQLiteStore) UpdateSyncConfig(ctx context.Context, config *models.SyncConfig) error {
	return s.db.WithContext(ctx).Save(config).Error
}

func (s *SQLiteStore) DeleteSyncConfig(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.SyncConfig{}, id).Error
}

// Sync state operations

func (s *SQLiteStore) CreateSyncState(ctx context.Context, state *models.SyncState) error {
	return s.db.WithContext(ctx).Create(state).Error
}

func (s *SQLiteStore) GetSyncState(ctx context.Context, syncConfigID uint, backendID, clientID string) (*models.SyncState, error) {
	var state models.SyncState
	err := s.db.WithContext(ctx).
		Where("sync_config_id = ? AND backend_id = ? AND client_id = ?", syncConfigID, backendID, clientID).
		First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *SQLiteStore) UpdateSyncState(ctx context.Context, state *models.SyncState) error {
	return s.db.WithContext(ctx).Save(state).Error
}

func (s *SQLiteStore) DeleteSyncState(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.SyncState{}, id).Error
}
