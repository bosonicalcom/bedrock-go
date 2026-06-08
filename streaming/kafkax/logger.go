package kafkax

import (
	"context"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Logger returns a logger adapter for kgo.Logger interface using [slog].
func Logger(lvl slog.Level, logger *slog.Logger) kgo.Logger {
	return loggerAdapter{lvl: lvl, slog: logger}
}

type loggerAdapter struct {
	lvl  slog.Level
	slog *slog.Logger
}

var (
	_toSlogLevelMap = map[kgo.LogLevel]slog.Level{
		kgo.LogLevelDebug: slog.LevelDebug,
		kgo.LogLevelInfo:  slog.LevelInfo,
		kgo.LogLevelWarn:  slog.LevelWarn,
		kgo.LogLevelError: slog.LevelError,
	}
	_fromSlogLevelMap = map[slog.Level]kgo.LogLevel{
		slog.LevelDebug: kgo.LogLevelDebug,
		slog.LevelInfo:  kgo.LogLevelInfo,
		slog.LevelWarn:  kgo.LogLevelWarn,
		slog.LevelError: kgo.LogLevelError,
	}

	_ kgo.Logger = (*loggerAdapter)(nil)
)

func (l loggerAdapter) Level() kgo.LogLevel {
	return _fromSlogLevelMap[l.lvl]
}

func (l loggerAdapter) Log(level kgo.LogLevel, msg string, keyvals ...any) {
	l.slog.Log(context.Background(), _toSlogLevelMap[level], msg, keyvals...)
}
