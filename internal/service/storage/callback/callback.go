package callback

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/x-thooh/delay/pkg/log"
)

type Payload struct {
	Schema string         `json:"schema,omitempty"`
	Url    string         `json:"url,omitempty"`
	Path   string         `json:"path,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

func (p Payload) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *Payload) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("payload should be []byte, got %T", src)
	}
	return json.Unmarshal(b, p)
}

type ICallback interface {
	SetLogger(lg log.Logger) ICallback
	Request(ctx context.Context, payload *Payload) (string, error)
	Close(ctx context.Context) error
}

var adapter = make(map[string]ICallback)

func GetAdapter(lg log.Logger) map[string]ICallback {
	for _, c := range adapter {
		c.SetLogger(lg)
	}
	return adapter
}

func RegisterAdapter(name string, c ICallback) {
	adapter[name] = c
}
