package storage

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/qustavo/sqlhooks/v2"
	"github.com/x-thooh/delay/internal/boot/database"
	"github.com/x-thooh/delay/internal/service/storage/callback"
	plog "github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/log/xslog"
)

func setupDB(lg plog.Logger) *sqlx.DB {
	dsn := "root:password@tcp(127.0.0.1:3306)/demo?parseTime=true&loc=Local"
	sql.Register("mysqlWithHooks", sqlhooks.Wrap(&mysql.MySQLDriver{}, &database.Hooks{Logger: lg}))
	db, err := sqlx.Connect("mysqlWithHooks", dsn)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func setConfig() *Config {
	return &Config{
		Debug:              true,
		Tick:               1 * time.Second,
		WheelSize:          60,
		PoolSize:           1000,
		Node:               0,
		PendingLimit:       10,
		PendingInterval:    3 * time.Second,
		AdvancePendingTime: 10 * time.Second,
		TimeoutLimit:       10,
		TimeoutInterval:    10 * time.Second,
		FastPathTime:       15 * time.Second,
	}
}

func setLogger() plog.Logger {
	lg, _, _ := xslog.New(&plog.Config{
		Model:      "std",
		Level:      "debug",
		Format:     "text",
		File:       "",
		MaxSizeMB:  0,
		MaxBackups: 0,
		MaxAgeDays: 0,
		Compress:   false,
	})
	return lg
}

func cleanupTasks(db *sqlx.DB) {
	db.Exec("DELETE FROM task_queue")
}

func TestAddImmediateTask(t *testing.T) {
	cfg := setConfig()
	lg := setLogger()
	db := setupDB(lg)
	ctx := context.Background()
	defer cleanupTasks(db)

	delay, err := New(cfg, lg, db)
	if err != nil {
		t.Fatal(err)
	}
	err = delay.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		delay.RegisterErrEvent(func(err error) {
			t.Logf("error event:%v", err)
		})
	}()
	defer delay.Stop(ctx)

	// 小于 15 秒延迟
	ret, err := delay.Add(ctx, WithDelayTime(5), WithBackoff(2, 16))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	t.Log("Inserted immediate task, wait 5 seconds to see execution", time.Now())
	time.Sleep(30 * time.Second)
}

func TestAddImmediateTask_SUCCESS(t *testing.T) {
	cfg := setConfig()
	lg := setLogger()
	db := setupDB(lg)
	ctx := context.Background()
	defer cleanupTasks(db)

	delay, err := New(cfg, lg, db)
	if err != nil {
		t.Fatal(err)
	}
	err = delay.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		delay.RegisterErrEvent(func(err error) {
			t.Logf("error event:%v", err)
		})
	}()
	defer delay.Stop(ctx)

	// 小于 15 秒延迟
	ret, err := delay.Add(ctx, WithPayload(&callback.Payload{
		Schema: "fmt",
		Data: map[string]any{
			"result": "SUCCESS",
		},
	}), WithDelayTime(5))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	t.Log("Inserted immediate task, wait 5 seconds to see execution", time.Now())
	time.Sleep(15 * time.Second)
}

func TestAddDelayTask(t *testing.T) {
	cfg := setConfig()
	lg := setLogger()
	db := setupDB(lg)
	ctx := context.Background()
	defer cleanupTasks(db)

	delay, err := New(cfg, lg, db)
	if err != nil {
		t.Fatal(err)
	}
	err = delay.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		delay.RegisterErrEvent(func(err error) {
			t.Logf("error event:%v", err)
		})
	}()
	defer delay.Stop(ctx)

	// 大于 15 秒延迟
	ret, err := delay.Add(ctx, WithDelayTime(20), WithBackoff(4, 25))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	t.Log("Inserted immediate task, wait 5 seconds to see execution", time.Now())
	time.Sleep(60 * time.Second)
}

func TestAddDelayTask_SUCCESS(t *testing.T) {
	cfg := setConfig()
	lg := setLogger()
	db := setupDB(lg)
	ctx := context.Background()
	defer cleanupTasks(db)

	delay, err := New(cfg, lg, db)
	if err != nil {
		t.Fatal(err)
	}
	err = delay.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		delay.RegisterErrEvent(func(err error) {
			t.Logf("error event:%v", err)
		})
	}()
	defer delay.Stop(ctx)

	// 大于 15 秒延迟
	ret, err := delay.Add(ctx, WithPayload(&callback.Payload{
		Schema: "fmt",
		Data: map[string]any{
			"result": "SUCCESS",
		},
	}), WithDelayTime(20))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	t.Log("Inserted immediate task, wait 5 seconds to see execution", time.Now())
	time.Sleep(30 * time.Second)
}

func TestAddDelayTask_GRPC(t *testing.T) {
	cfg := setConfig()
	lg := setLogger()
	db := setupDB(lg)
	ctx := context.Background()
	defer cleanupTasks(db)

	delay, err := New(cfg, lg, db)
	if err != nil {
		t.Fatal(err)
	}
	err = delay.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		delay.RegisterErrEvent(func(err error) {
			t.Logf("error event:%v", err)
		})
	}()
	defer delay.Stop(ctx)

	// 大于 15 秒延迟
	ret, err := delay.Add(ctx, WithPayload(&callback.Payload{
		Schema: "grpc",
		Url:    "127.0.0.1:50051",
		Path:   "example.Example/Valid",
		Data: map[string]any{
			"result": "SUCCESS",
		},
	}), WithDelayTime(20))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	t.Log("Inserted immediate task, wait 5 seconds to see execution", time.Now())
	time.Sleep(30 * time.Second)
}

func TestAddDelayTask_HTTP(t *testing.T) {
	cfg := setConfig()
	lg := setLogger()
	db := setupDB(lg)
	ctx := context.Background()
	defer cleanupTasks(db)

	delay, err := New(cfg, lg, db)
	if err != nil {
		t.Fatal(err)
	}
	err = delay.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		delay.RegisterErrEvent(func(err error) {
			t.Logf("error event:%v", err)
		})
	}()
	defer delay.Stop(ctx)

	// 小于 15 秒延迟
	ret, err := delay.Add(ctx, WithPayload(&callback.Payload{
		Schema: "http",
		Url:    "127.0.0.1:8081",
		Path:   "/example/valid",
		Data: map[string]any{
			"result": "SUCCESS",
		},
	}), WithDelayTime(5))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)

	t.Log("Inserted immediate task, wait 5 seconds to see execution", time.Now())
	time.Sleep(15 * time.Second)
}
