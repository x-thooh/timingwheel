package logger

import (
	"context"

	"github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/log/xslog"
)

// InitLogger 日志
func InitLogger(cfg *log.Config) (log.Logger, func(), error) {
	return xslog.New(cfg)
}

type DefaultLogger struct {
	Lg log.Logger
}

func (l *DefaultLogger) Debugf(format string, a ...interface{}) {
	l.Lg.Debug(context.Background(), format, a...)
}

func (l *DefaultLogger) Infof(format string, a ...interface{}) {
	l.Lg.Info(context.Background(), format, a...)
}

func (l *DefaultLogger) Warnf(format string, a ...interface{}) {
	l.Lg.Warn(context.Background(), format, a...)
}

func (l *DefaultLogger) Errorf(format string, a ...interface{}) {
	l.Lg.Error(context.Background(), format, a...)
}
