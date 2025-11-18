package config

import (
	"fmt"
	"os"

	"github.com/x-thooh/delay/internal/boot/database"
	"github.com/x-thooh/delay/internal/service/storage"
	"gopkg.in/yaml.v3"
)

func LoadConfig(path string, env string) (*Entity, error) {
	path = fmt.Sprintf("%s/configs.%s.yaml", path, env)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Entity{
		Base: &Base{
			Env: env,
		},
		Database: &database.Config{
			Debug: "dev" == env,
		},
		TimingWheel: &storage.Config{
			Debug: "dev" == env,
		},
	}
	if err = yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
