package log

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/mwantia/fabric/pkg/container"
)

// LoggerTagProcessor handles fabric:"logger" and fabric:"logger:<name>" tags
// for automatic logger injection with optional named loggers.
//
// Supported tag formats:
//   - `fabric:"logger"` - Injects the base logger service
//   - `fabric:"logger:<name>"` - Injects a named logger (e.g., logger.Named("database"))
type LoggerTagProcessor struct{}

// NewLoggerTagProcessor creates a new LoggerTagProcessor instance.
func NewLoggerTagProcessor() *LoggerTagProcessor {
	return &LoggerTagProcessor{}
}

// GetPriority returns the processing priority for this processor.
// Priority 50 ensures it runs before the default inject processor (priority 0)
// but after any custom high-priority processors.
func (ltp *LoggerTagProcessor) GetPriority() int {
	return 50
}

// CanProcess returns true if this processor can handle the given tag value.
// The LoggerTagProcessor handles:
//   - "logger" - for base logger injection
//   - "logger:<name>" - for named logger injection
//
// All matching is case-insensitive.
func (ltp *LoggerTagProcessor) CanProcess(value string) bool {
	return strings.EqualFold(value, "logger") || strings.HasPrefix(strings.ToLower(value), "logger:")
}

// Process handles the injection of loggers for fabric:"logger" tags.
// It supports both base and named logger injection:
//   - "logger" - resolves the base LoggerService
//   - "logger:<name>" - resolves the base LoggerService and calls Named(name)
//
// The method parses the tag value to extract the logger name and then
// resolves the appropriate logger from the container.
func (ltp *LoggerTagProcessor) Process(ctx context.Context, sc *container.ServiceContainer, field reflect.StructField, value string) (any, error) {
	// First, resolve the base logger service from the container
	ok, resolved := sc.ResolveByType(ctx, reflect.TypeOf((*LoggerService)(nil)).Elem())
	if !ok {
		return nil, fmt.Errorf("failed to resolve LoggerService for field '%s': no logger service registered", field.Name)
	}

	baseLogger, ok := resolved.(LoggerService)
	if !ok {
		return nil, fmt.Errorf("resolved logger is not a LoggerService for field '%s'", field.Name)
	}

	// Parse the tag value to extract the logger name
	loggerName := ""
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			loggerName = strings.TrimSpace(parts[1])
		}
	}

	// If a name is specified, create a named logger
	if loggerName != "" {
		return baseLogger.Named(loggerName), nil
	}

	// Otherwise, return the base logger
	return baseLogger, nil
}
