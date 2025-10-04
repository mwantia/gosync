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
	"github.com/mwantia/gosync/pkg/log"
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

	gsa.log.Debug("Registering 'LoggerService'...")
	errs.Add(container.Register[log.LoggerServiceImpl](gsa.sc,
		container.With[log.LoggerService](),
		container.WithInstance(gsa.log)))

	return errs.Errors()
}

func (gsa *GoSyncAgent) Serve(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	gsa.mutex.Lock()

	if err := gsa.setupServices(); err != nil {
		return err
	}

	gsa.mutex.Unlock()
	<-ctx.Done()

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
