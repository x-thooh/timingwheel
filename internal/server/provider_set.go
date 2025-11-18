package server

import (
	"github.com/google/wire"
	"github.com/x-thooh/delay/internal/server/grpc"
	"github.com/x-thooh/delay/internal/server/http"
)

var ProviderSetServer = wire.NewSet(
	http.New,
	grpc.New,
)
