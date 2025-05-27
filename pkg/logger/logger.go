package logger

import (
	"fmt"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/internal/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	once   sync.Once
	logger zerolog.Logger
)

func getLogPath() string {
	cfg := config.Get()
	logsDir := filepath.Join(cfg.Path, "logs")

	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			panic(fmt.Sprintf("Failed to create logs directory: %v", err))
		}
	}

	return filepath.Join(logsDir, "scroblarr.log")
}

func NewLogger(prefix string) zerolog.Logger {
	cfg := config.Get()

	rotatingLogFile := &lumberjack.Logger{
		Filename:   getLogPath(),
		MaxSize:    2,
		MaxBackups: 2,
		MaxAge:     28,
		Compress:   true,
	}

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    false, // Set to true if you don't want colors
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("[%s] %v", prefix, i)
		},
	}

	fileWriter := zerolog.ConsoleWriter{
		Out:        rotatingLogFile,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    true, // No colors in file output
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("[%s] %v", prefix, i)
		},
	}

	multi := zerolog.MultiLevelWriter(consoleWriter, fileWriter)

	logger := zerolog.New(multi).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)

	// Set the log level
	switch cfg.LogLevel {
	case "debug":
		logger = logger.Level(zerolog.DebugLevel)
	case "info":
		logger = logger.Level(zerolog.InfoLevel)
	case "warn":
		logger = logger.Level(zerolog.WarnLevel)
	case "error":
		logger = logger.Level(zerolog.ErrorLevel)
	default:
		logger = logger.Level(zerolog.InfoLevel)
	}
	return logger
}

func GetDefault() zerolog.Logger {
	once.Do(func() {
		logger = NewLogger("scroblarr")
	})
	return logger
}
