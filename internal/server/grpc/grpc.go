package grpc

import (
	"context"
	"fmt"
	"net"

	pbdelay "github.com/x-thooh/delay/api/delay"
	pbexample "github.com/x-thooh/delay/api/example"
	middleware "github.com/x-thooh/delay/internal/server/grpc/middleware"
	"github.com/x-thooh/delay/pkg/log"
	"google.golang.org/grpc"
)

type Server struct {
	cfg           *Config
	lg            log.Logger
	gs            *grpc.Server
	delayServer   pbdelay.DelayServer
	exampleServer pbexample.ExampleServer
}

func New(
	cfg *Config,
	lg log.Logger,
	delayServer pbdelay.DelayServer,
	exampleServer pbexample.ExampleServer,
) *Server {
	s := &Server{
		cfg:           cfg,
		lg:            lg,
		delayServer:   delayServer,
		exampleServer: exampleServer,
	}
	return s
}

type Config struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func (s *Server) Start(_ context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port))
	if err != nil {
		return err
	}

	s.gs = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.UnaryServerLogInterceptor(s.lg),
			middleware.UnaryServerValidatorInterceptor(s.lg),
		),
		grpc.ChainStreamInterceptor(
			middleware.StreamServerLogInterceptor(s.lg),
			middleware.StreamServerValidatorInterceptor(s.lg),
		),
	)
	pbdelay.RegisterDelayServer(s.gs, s.delayServer)
	pbexample.RegisterExampleServer(s.gs, s.exampleServer)

	return s.gs.Serve(lis)
}

func (s *Server) Stop(_ context.Context) error {
	s.gs.GracefulStop()
	return nil
}
