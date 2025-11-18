package example

import (
	"context"

	pbexample "github.com/x-thooh/delay/api/example"
	"github.com/x-thooh/delay/internal/service/storage"
	"github.com/x-thooh/delay/pkg/log"
	"google.golang.org/protobuf/types/known/structpb"
)

type service struct {
	pbexample.UnimplementedExampleServer

	lg log.Logger

	storage *storage.Storage
}

func New(
	lg log.Logger,
) pbexample.ExampleServer {
	s := &service{
		lg: lg,
	}
	return s
}

func (s *service) Valid(ctx context.Context, req *structpb.Struct) (*structpb.Value, error) {
	m := req.AsMap()
	s.lg.Info(ctx, "req is map", "m", m)
	ret, ok := m["result"]
	if !ok {
		return structpb.NewStringValue("fail: not map"), nil
	}
	str, ok := ret.(string)
	if !ok {
		return structpb.NewStringValue("fail: not string"), nil
	}
	return structpb.NewStringValue(str), nil
}
