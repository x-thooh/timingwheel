package xslog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/trace"
)

type xslog struct {
	logger *slog.Logger
}

func New(
	cfg *log.Config,
) (log.Logger, func(), error) {

	var (
		writers       []io.Writer
		fileWriter    *lumberjack.Logger
		consoleWriter *os.File
	)
	for _, model := range strings.Split(cfg.Model, ",") {
		switch model {
		case "file":
			// 文件输出
			fileWriter = &lumberjack.Logger{
				Filename:   cfg.File,
				MaxSize:    cfg.MaxSizeMB,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAgeDays,
				Compress:   cfg.Compress,
			}
			writers = append(writers, fileWriter)
		default:
			consoleWriter = os.Stdout
			writers = append(writers, consoleWriter)
		}
	}

	multiWriter := io.MultiWriter(writers...)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(cfg.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				// 将 time.Time 转成毫秒整数
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(a.Key, t.Format("2006-01-02T15:04:05.000Z07:00"))
				}
			}
			// SQL 高亮只在控制台
			if a.Key == "sql" && consoleWriter != nil {
				if s, ok := a.Value.Any().(string); ok {
					return slog.String(a.Key, colorSQL(s))
				}
			}
			return a
		},
	}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(multiWriter, opts)
	default:
		handler = NewANSIColorHandler(multiWriter, opts, true)
		// handler = slog.NewTextHandler(multiWriter, opts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return &xslog{
			logger: l,
		}, func() {
			if fileWriter != nil {
				fileWriter.Rotate()
			}
		}, nil
}

func (x *xslog) Debug(ctx context.Context, msg string, args ...any) {
	x.common(ctx).DebugContext(ctx, msg, args...)
}

func (x *xslog) Info(ctx context.Context, msg string, args ...any) {
	x.common(ctx).InfoContext(ctx, msg, args...)
}

func (x *xslog) Warn(ctx context.Context, msg string, args ...any) {
	x.common(ctx).WarnContext(ctx, msg, args...)
}

func (x *xslog) Error(ctx context.Context, msg string, args ...any) {
	x.common(ctx).ErrorContext(ctx, msg, args...)
}

func (x *xslog) common(ctx context.Context) *slog.Logger {
	return x.logger.With(slog.String(trace.GetCtxKey(), trace.Get(ctx)))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func GetCtxTraceID(ctx context.Context) string {
	traceId := ""
	switch ctx.(type) {
	default:
		traceId = trace.Get(ctx)
	}
	if len(traceId) == 0 {
		traceId = trace.GenerateTraceID()
	}
	return traceId
}
