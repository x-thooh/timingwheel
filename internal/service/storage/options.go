package storage

import (
	"github.com/x-thooh/delay/internal/service/storage/callback"
)

type options struct {
	// 延迟时间，最大30s
	delayTime int64
	// 超时时间
	timeout int64
	// 重试时间
	backoff []int64

	// 定时
	cron string

	// 回调
	payload *callback.Payload
}

type Option func(*options)

func WithDelayTime(d int64) Option {
	return func(o *options) {
		o.delayTime = d
	}
}

func WithTimeout(timeout int64) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

func WithBackoff(b ...int64) Option {
	return func(o *options) {
		o.backoff = b
	}
}

func WithCron(cron string) Option {
	return func(o *options) {
		o.cron = cron
	}
}

func WithPayload(payload *callback.Payload) Option {
	return func(o *options) {
		o.payload = payload
	}
}
