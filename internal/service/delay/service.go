package delay

import (
	"context"

	pbdelay "github.com/x-thooh/delay/api/delay"
	"github.com/x-thooh/delay/internal/service/storage"
	"github.com/x-thooh/delay/internal/service/storage/callback"
)

type service struct {
	pbdelay.UnimplementedDelayServer

	storage *storage.Storage
}

func New(
	storage *storage.Storage,
) pbdelay.DelayServer {
	s := &service{
		storage: storage,
	}
	return s
}

func (s *service) Register(ctx context.Context, request *pbdelay.RegisterRequest) (*pbdelay.RegisterReply, error) {
	tn, err := s.storage.Add(
		ctx,
		storage.WithDelayTime(request.GetDelayTime()),
		storage.WithTimeout(request.GetTimeout()),
		storage.WithBackoff(request.GetBackoff()...),

		storage.WithPayload(&callback.Payload{
			Schema: request.GetSchema(),
			Url:    request.GetUrl(),
			Path:   request.GetPath(),
			Data:   request.GetData().AsMap(),
		}),
	)
	if err != nil {
		return nil, err
	}
	return &pbdelay.RegisterReply{TaskNo: tn}, nil
}
