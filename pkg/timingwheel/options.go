package timingwheel

import (
	"time"

	"github.com/panjf2000/ants"
)

type Option func(opts *Options)

type Options struct {
	aos []ants.Option

	poolSize int

	timeout time.Duration
}

func WithAntsOption(aos ...ants.Option) Option {
	return func(opts *Options) {
		opts.aos = aos
	}
}

func WithPoolSize(poolSize int) Option {
	return func(opts *Options) {
		opts.poolSize = poolSize
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.timeout = timeout
	}
}
