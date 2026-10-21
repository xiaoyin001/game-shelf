// 文件说明：封装项目统一的 slog JSON 日志初始化和文件输出逻辑。
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"game-shelf/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

func New(cfg config.Log) (*slog.Logger, io.Closer, error) {
	writer, closer, err := newWriter(cfg)
	if err != nil {
		return nil, nil, err
	}
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:       parseLevel(cfg.Level),
		ReplaceAttr: replaceAttr,
	})), closer, nil
}

func MustNew(cfg config.Log) (*slog.Logger, io.Closer) {
	log, closer, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return log, closer
}

func Discard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newWriter(cfg config.Log) (io.Writer, io.Closer, error) {
	if strings.EqualFold(cfg.Output, "file") {
		if cfg.FilePath == "" {
			return nil, nil, fmt.Errorf("log.file_path is required when log.output=file")
		}
		if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0o755); err != nil {
			return nil, nil, err
		}
		logger := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    valueOrDefault(cfg.MaxSizeMB, 100),
			MaxBackups: valueOrDefault(cfg.MaxBackups, 7),
			MaxAge:     valueOrDefault(cfg.MaxAgeDays, 14),
			Compress:   cfg.Compress,
		}
		return logger, logger, nil
	}
	return os.Stdout, nil, nil
}

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		a.Key = "ts"
	}
	if a.Key == slog.MessageKey {
		a.Key = "msg"
	}
	return a
}

func valueOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
