package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/x-thooh/delay/internal/boot/logger"
	"github.com/x-thooh/delay/internal/config"
	sg "github.com/x-thooh/delay/internal/server/grpc"
	sh "github.com/x-thooh/delay/internal/server/http"
	"github.com/x-thooh/delay/internal/service/storage"
	"github.com/x-thooh/delay/pkg/app"
	"github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/trace"
	"github.com/x-thooh/delay/pkg/util"
)

// go build -ldflags "-x main.Version=x.y.z"
var (
	name = "delay"
	env  = "prod"
	conf = "../../configs"
)

func newApp(lg log.Logger, h *sh.Server, g *sg.Server, s *storage.Storage) *app.App {
	return app.New(
		app.Context(trace.Set(context.Background(), trace.GenerateTraceID())),
		app.Metadata(map[string]string{}),
		app.Logger(&logger.DefaultLogger{Lg: lg}),
		app.Server(
			h,
			g,
			s,
		),
	)
}

func main() {
	flag.StringVar(&env, "env", "prod", "env: dev, test, prod")
	flag.StringVar(&conf, "conf", fmt.Sprintf("../../../%s/configs", name), "path: ../../configs")
	flag.Parse()
	fmt.Println(os.Args, env, conf)
	cfgEntity, err := config.LoadConfig(util.AbPath(conf), env)
	if err != nil {
		panic(err)
	}

	ap, fn, err := wireApp(cfgEntity)
	if err != nil {
		panic(err)
	}
	defer func() {
		fn()
	}()

	if err = ap.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}

}
