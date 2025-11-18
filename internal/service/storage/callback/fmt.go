package callback

import (
	"context"

	"github.com/x-thooh/delay/pkg/log"
)

type Fmt struct {
	lg log.Logger
}

func init() {
	RegisterAdapter("FMT", NewFmt())
}

func NewFmt() ICallback {
	return &Fmt{}
}

func (f *Fmt) SetLogger(lg log.Logger) ICallback {
	f.lg = lg
	return f
}

func (f *Fmt) Request(ctx context.Context, payload *Payload) (string, error) {
	if ret, ok := payload.Data["result"]; ok {
		s, ok1 := ret.(string)
		if ok1 {
			return s, nil
		}

	}
	return "FAIL", nil
}

func (f *Fmt) Close(ctx context.Context) error {
	return nil
}
