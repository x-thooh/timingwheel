package config

import (
	"github.com/x-thooh/delay/internal/boot/database"
	"github.com/x-thooh/delay/internal/server/grpc"
	"github.com/x-thooh/delay/internal/server/http"
	"github.com/x-thooh/delay/internal/service/storage"
	"github.com/x-thooh/delay/pkg/log"
)

type Entity struct {
	*Base
	Logger      *log.Config      `yaml:"logger"`
	HTTP        *http.Config     `yaml:"http"`
	GRPC        *grpc.Config     `yaml:"grpc"`
	Database    *database.Config `yaml:"database"`
	TimingWheel *storage.Config  `yaml:"timingwheel"`
}

type Base struct {
	Env string `yaml:"env"`
}

func RegisterLogger(entity *Entity) *log.Config {
	return entity.Logger
}

func RegisterHTTP(entity *Entity) *http.Config {
	return entity.HTTP
}

func RegisterGRPC(entity *Entity) *grpc.Config {
	return entity.GRPC
}

func RegisterDatabase(entity *Entity) *database.Config {
	return entity.Database
}

func RegisterTimingWheel(entity *Entity) *storage.Config {
	return entity.TimingWheel
}
