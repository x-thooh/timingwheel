package callback

import (
	"context"
	"sync"

	"github.com/x-thooh/delay/pkg/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

type GRPC struct {
	mu      sync.Mutex
	clients map[string]*grpc.ClientConn
	lg      log.Logger
}

func init() {
	RegisterAdapter("GRPC", NewGRPC())
}

func NewGRPC() ICallback {
	return &GRPC{
		clients: make(map[string]*grpc.ClientConn),
	}
}

func (g *GRPC) SetLogger(lg log.Logger) ICallback {
	g.lg = lg
	return g
}

func (g *GRPC) getClient(url string) (*grpc.ClientConn, error) {
	g.mu.Lock()
	if cc, ok := g.clients[url]; ok {
		g.mu.Unlock()
		return cc, nil
	}
	g.mu.Unlock()

	cc, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	g.clients[url] = cc
	return cc, nil
}

func (g *GRPC) Request(ctx context.Context, payload *Payload) (string, error) {
	cc, err := g.getClient(payload.Url)
	if err != nil {
		return "", err
	}

	args, err := structpb.NewStruct(payload.Data)
	if err != nil {
		return "", err
	}

	reply := new(structpb.Value)
	if err = cc.Invoke(ctx, payload.Path, args, reply); err != nil {
		return "", err
	}

	return reply.GetStringValue(), nil
}

func (g *GRPC) Close(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, c := range g.clients {
		if err := c.Close(); err != nil {
			return err
		}
	}
	return nil
}
