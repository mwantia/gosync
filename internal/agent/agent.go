package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/mwantia/fabric/pkg/container"
	config "github.com/mwantia/gosync/internal/config/server"
	"github.com/mwantia/gosync/pkg/db/migrations"
	"github.com/mwantia/gosync/pkg/db/store"
	"github.com/mwantia/gosync/pkg/log"
	"gorm.io/gorm/logger"
)

type GoSyncAgent struct {
	mutex sync.RWMutex
	wait  sync.WaitGroup

	cfg *config.BaseServerConfig
	sc  *container.ServiceContainer
	log log.LoggerService
}

func NewAgent(cfg *config.BaseServerConfig) *GoSyncAgent {
	return &GoSyncAgent{
		cfg: cfg,
		sc:  container.NewServiceContainer(),
		log: log.NewLoggerService("golang", cfg.Log),
	}
}

func (gsa *GoSyncAgent) setupServices() error {
	errs := container.Errors{}

	gsa.sc.AddTagProcessor(log.NewLoggerTagProcessor())

	gsa.log.Debug("Registering 'LoggerService'...")
	errs.Add(container.Register[*log.LoggerServiceImpl](gsa.sc,
		container.With[log.LoggerService](),
		container.WithInstance(gsa.log)))

	gsa.log.Debug("Registering 'MetadataStore'...")
	errs.Add(container.Register[store.MetadataStore](gsa.sc,
		container.AsFactory(func(ctx context.Context, sc *container.ServiceContainer) (any, error) {
			return gsa.initMetadataStore()
		})))

	return errs.Errors()
}

func (gsa *GoSyncAgent) initMetadataStore() (store.MetadataStore, error) {
	switch gsa.cfg.Metadata.Type {
	case "sqlite":
		gsa.log.Info("Initializing SQLite metadata store at %s", gsa.cfg.Metadata.SQLite.Path)

		// Determine log level for GORM
		gormLogLevel := logger.Silent
		if gsa.cfg.Log.Level == "DEBUG" {
			gormLogLevel = logger.Info
		}

		sqliteStore, err := store.NewSQLiteStore(store.SQLiteConfig{
			Path:     gsa.cfg.Metadata.SQLite.Path,
			LogLevel: gormLogLevel,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create sqlite store: %w", err)
		}

		// Connect to database
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := sqliteStore.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		// Run migrations
		gsa.log.Info("Running database migrations...")
		migrator := migrations.NewMigrator(sqliteStore.DB())
		if err := migrator.Migrate(ctx); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		gsa.log.Info("Metadata store initialized successfully")
		return sqliteStore, nil

	default:
		return nil, fmt.Errorf("unsupported metadata store type: %s", gsa.cfg.Metadata.Type)
	}
}

func (gsa *GoSyncAgent) Serve(ctx context.Context) error {
	gsa.log.Info("Starting GoSync Agent...")

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	gsa.mutex.Lock()

	gsa.log.Debug("Setting up services...")
	if err := gsa.setupServices(); err != nil {
		gsa.log.Error("Failed to setup services: %v", err)
		return err
	}

	gsa.mutex.Unlock()
	gsa.log.Info("GoSync Agent started successfully. Press Ctrl+C to stop.")
	<-ctx.Done()
	gsa.log.Info("Shutdown signal received...")

	timeout, err := time.ParseDuration(gsa.cfg.ShutdownTimeout)
	if err != nil {
		// Set default of 60 seconds if error
		timeout = 60 * time.Second
	}

	shutdown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := gsa.sc.Cleanup(shutdown); err != nil {
		return fmt.Errorf("failed to complete service container cleanup: %w", err)
	}

	gsa.wait.Wait()
	return nil
}
