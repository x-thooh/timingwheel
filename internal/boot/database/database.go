package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql" // MySQL 驱动
	"github.com/jmoiron/sqlx"
	"github.com/qustavo/sqlhooks/v2"
	"github.com/x-thooh/delay/pkg/log"
)

type Config struct {
	Debug           bool
	Driver          string `yaml:"driver"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Name            string `yaml:"name"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime string `yaml:"conn_max_idle_time"`
}

// InitSQLX 初始化数据库连接
func InitSQLX(lg log.Logger, cfg *Config) (*sqlx.DB, error) {

	if cfg.Debug {
		sql.Register("mysqlWithHooks", sqlhooks.Wrap(&mysql.MySQLDriver{}, &Hooks{lg}))

		cfg.Driver = fmt.Sprintf("%sWithHooks", cfg.Driver)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	db, err := sqlx.Open(cfg.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 连接池配置
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)

	if d, pErr := time.ParseDuration(cfg.ConnMaxLifetime); pErr == nil {
		db.SetConnMaxLifetime(d)
	}

	if d, pErr := time.ParseDuration(cfg.ConnMaxIdleTime); pErr == nil {
		db.SetConnMaxIdleTime(d)
	}

	// 检测连接是否正常
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
