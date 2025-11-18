//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/google/wire"
	"github.com/x-thooh/delay/internal/boot/database"
	"github.com/x-thooh/delay/internal/boot/logger"
	"github.com/x-thooh/delay/internal/config"
	"github.com/x-thooh/delay/internal/server"
	"github.com/x-thooh/delay/internal/service"
	"github.com/x-thooh/delay/pkg/app"
)

// wireApp init app application.
func wireApp(*config.Entity) (*app.App, func(), error) {
	panic(wire.Build(
		config.ProviderSetConfig,
		logger.InitLogger,
		database.InitSQLX,
		service.ProviderSetService,
		server.ProviderSetServer,
		newApp,
	))
}
