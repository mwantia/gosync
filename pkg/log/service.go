package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	config "github.com/mwantia/gosync/internal/config/server"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LoggerService interface {
	Debug(msg string, args ...any)

	Info(msg string, args ...any)

	Warn(msg string, args ...any)

	Error(msg string, args ...any)

	Fatal(msg string, args ...any)

	Named(name string) LoggerService
}

type LoggerServiceImpl struct {
	LoggerService

	cfg    config.LogServerConfig
	name   string
	level  LogLevel
	writer io.Writer
}

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Service   string `json:"service,omitempty"`
	Message   string `json:"message"`
}

func NewLoggerService(name string, cfg config.LogServerConfig) LoggerService {
	level := Parse(cfg.Level)

	impl := &LoggerServiceImpl{
		cfg:   cfg,
		name:  name,
		level: level,
	}

	impl.setupWriter()
	return impl
}

func (impl *LoggerServiceImpl) setupWriter() {
	var writers []io.Writer

	if !impl.cfg.NoTerminal {
		writers = append(writers, os.Stdout)
	}

	if impl.cfg.File != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   impl.cfg.File,
			MaxSize:    impl.cfg.Rotation.MaxSize,
			MaxBackups: impl.cfg.Rotation.MaxBackups,
			MaxAge:     impl.cfg.Rotation.MaxAge,
			Compress:   impl.cfg.Rotation.Compress,
		}
		writers = append(writers, fileWriter)
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	impl.writer = io.MultiWriter(writers...)
}

func (impl *LoggerServiceImpl) log(level LogLevel, msg string, args ...any) {
	if level < impl.level {
		return
	}

	timestamp := time.Now().Format(impl.cfg.TimeFormat)
	formattedMsg := fmt.Sprintf(msg, args...)

	if impl.cfg.JSON {
		entry := logEntry{
			Timestamp: timestamp,
			Level:     level.String(),
			Message:   formattedMsg,
		}
		if impl.name != "" {
			entry.Service = impl.name
		}

		jsonBytes, _ := json.Marshal(entry)
		fmt.Fprintf(impl.writer, "%s\n", jsonBytes)
	} else {
		prefix := fmt.Sprintf("[%s] %-5s", timestamp, level)
		if impl.name != "" {
			prefix = fmt.Sprintf("%s [%s]", prefix, impl.name)
		}

		if !impl.cfg.NoTerminal && !impl.cfg.NoColor {
			fmt.Fprintf(impl.writer, "%s%s %s\033[0m\n", Color(level), prefix, formattedMsg)
		} else {
			fmt.Fprintf(impl.writer, "%s %s\n", prefix, formattedMsg)
		}
	}

	if level == Fatal {
		os.Exit(1)
	}
}

func (impl *LoggerServiceImpl) Debug(msg string, args ...any) {
	impl.log(Debug, msg, args...)
}

func (impl *LoggerServiceImpl) Info(msg string, args ...any) {
	impl.log(Info, msg, args...)
}

func (impl *LoggerServiceImpl) Warn(msg string, args ...any) {
	impl.log(Warn, msg, args...)
}

func (impl *LoggerServiceImpl) Error(msg string, args ...any) {
	impl.log(Error, msg, args...)
}

func (impl *LoggerServiceImpl) Fatal(msg string, args ...any) {
	impl.log(Fatal, msg, args...)
}

func (impl *LoggerServiceImpl) Named(name string) LoggerService {
	return &LoggerServiceImpl{
		cfg:    impl.cfg,
		name:   fmt.Sprintf("%s/%s", impl.name, name),
		level:  impl.level,
		writer: impl.writer, // Share the same writer
	}
}
