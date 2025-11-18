package config

import (
	"github.com/google/wire"
)

var ProviderSetConfig = wire.NewSet(
	RegisterLogger,
	RegisterHTTP,
	RegisterGRPC,
	RegisterDatabase,
	RegisterTimingWheel,
)
