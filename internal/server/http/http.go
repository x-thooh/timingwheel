package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pbdelay "github.com/x-thooh/delay/api/delay"
	pbexample "github.com/x-thooh/delay/api/example"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

type Server struct {
	cfg *Config
	srv *http.Server
}

type Config struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`

	GHost string `yaml:"ghost"`
	GPort int    `yaml:"gport"`
}

func New(
	cfg *Config,
) *Server {
	s := &Server{
		cfg: cfg,
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &plainTextMarshaler{
		JSONPb: runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true, // 原来 OrigName
				EmitUnpopulated: true, // 原来 EmitDefaults
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		},
	}))

	// 连接到 gRPC 后端
	err := pbdelay.RegisterDelayHandlerFromEndpoint(
		ctx, mux, fmt.Sprintf("%s:%d", s.cfg.GHost, s.cfg.GPort),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)
	if err != nil {
		return err
	}
	err = pbexample.RegisterExampleHandlerFromEndpoint(
		ctx, mux, fmt.Sprintf("%s:%d", s.cfg.GHost, s.cfg.GPort),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)
	if err != nil {
		return err
	}

	// 创建 http.Server
	s.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port),
		Handler: mux,
	}

	// 运行 HTTP 服务（阻塞）
	return s.srv.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		// 优雅关闭，等待正在处理的请求完成
		if err := s.srv.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}
