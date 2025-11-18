package storage

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/jmoiron/sqlx"
	"github.com/panjf2000/ants"
	"github.com/x-thooh/delay/internal/service/storage/callback"
	"github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/timingwheel"
	"github.com/x-thooh/delay/pkg/trace"
)

type Storage struct {
	cfg *Config
	lg  log.Logger
	db  *sqlx.DB
	sn  *snowflake.Node
	tw  *timingwheel.TimingWheel

	ch chan error

	adapter map[string]callback.ICallback

	ns []int
}

type Config struct {
	Debug     bool
	Tick      time.Duration `yaml:"tick"`
	WheelSize int64         `yaml:"wheel_size"`
	PoolSize  int           `yaml:"pool_size"`

	Node      int    `yaml:"node"`
	NameSpace string `yaml:"name_space"`
	StsName   string `yaml:"sts_name"`

	PendingLimit       int           `yaml:"pending_limit"`
	PendingInterval    time.Duration `yaml:"pending_interval"`
	AdvancePendingTime time.Duration `yaml:"advance_pending_time"`

	TimeoutLimit    int           `yaml:"timeout_limit"`
	TimeoutInterval time.Duration `yaml:"timeout_interval"`

	NodeInterval time.Duration `yaml:"node_interval"`

	FastPathTime time.Duration `yaml:"fast_path_time"`
}

func New(
	cfg *Config,
	lg log.Logger,
	db *sqlx.DB,
) (*Storage, error) {
	tw, err := timingwheel.New(
		cfg.Tick,
		cfg.WheelSize,
		timingwheel.WithPoolSize(cfg.PoolSize),
		timingwheel.WithAntsOption(
			ants.WithPreAlloc(true),
			ants.WithExpiryDuration(31*time.Second),
		),
	)
	if err != nil {
		return nil, err
	}
	sn, err := snowflake.NewNode(int64(cfg.Node))
	if err != nil {
		return nil, err
	}
	d := &Storage{
		cfg:     cfg,
		lg:      lg,
		db:      db,
		sn:      sn,
		tw:      tw,
		adapter: callback.GetAdapter(lg),
	}

	d.SetNodes([]int{cfg.Node})
	return d, nil
}

func (d *Storage) RegisterErrEvent(cb func(err error)) {
	if d.ch == nil {
		d.ch = make(chan error, 100)
	}
	for err := range d.ch {
		cb(err)
	}
}

func (d *Storage) collect(ctx context.Context, err error) {
	select {
	case d.ch <- err:
	default:
		d.lg.Error(ctx, "error dropped", "err", err)
	}
}

func (d *Storage) SetNodes(ns []int) {
	d.lg.Info(context.Background(), "set nodes", "nodes", ns, "node", d.cfg.Node)
	// 存活节点
	if len(ns) == 0 {
		ns = []int{d.cfg.Node}
	}
	ns = append([]int{-1}, ns...)
	for i := 0; i < len(ns); i++ {
		if ns[i] >= d.cfg.Node {
			end := ns[i]
			if i == len(ns)-1 {
				end = math.MaxInt64
			}
			d.ns = []int{ns[i-1], end}
			break
		}
	}
	d.lg.Info(context.Background(), "set nodes", "ns", d.ns)
}

func (d *Storage) Start(_ context.Context) error {
	// 待处理
	if err := d.ScheduleFunc(d.cfg.PendingInterval, func(ctx context.Context) {
		d.lg.Debug(ctx, "Cron Pending Start")
		defer func() {
			d.lg.Debug(ctx, "Cron Pending End")
		}()
		pendingTasks, fErr := d.FetchPendingTasks(ctx, d.cfg.PendingLimit, d.cfg.AdvancePendingTime)
		if fErr != nil {
			fErr = fmt.Errorf("fetch pending tasks: %w", fErr)
			d.collect(ctx, fErr)
			return
		}
		d.lg.Debug(ctx, "Cron Pending", slog.Any("tasks", pendingTasks))
		for _, task := range pendingTasks {
			task.FailCount++
			if err := d.Submit(trace.Append(ctx, task.TraceId()), task, 0); err != nil {
				err = fmt.Errorf("execute task %d: %w", task.TaskNo, err)
				d.collect(ctx, err)
				continue
			}
		}
	}); err != nil {
		return err
	}

	// // 超时
	// if err := d.ScheduleFunc(d.cfg.TimeoutInterval, func(ctx context.Context) {
	// 	d.lg.Debug(ctx, "Cron Timeout Start")
	// 	defer func() {
	// 		d.lg.Debug(ctx, "Cron Timeout End")
	// 	}()
	// 	timeoutTasks, fErr := d.FetchTimeoutTasks(ctx, d.cfg.TimeoutLimit)
	// 	if fErr != nil {
	// 		fErr = fmt.Errorf("fetch timeout tasks: %w", fErr)
	// 		d.collect(ctx, fErr)
	// 		return
	// 	}
	// 	d.lg.Debug(ctx, "Cron Timeout", slog.Any("tasks", timeoutTasks))
	// 	for _, task := range timeoutTasks {
	// 		if err := d.Failure(trace.Append(ctx, task.TraceId()), task.WithFailMsg(&FailMsg{
	// 			Resp: "",
	// 			Err:  fmt.Sprintf("task timeout, timtout:%vs", task.Timeout),
	// 		})); err != nil {
	// 			err = fmt.Errorf("fail task %d: %w", task.TaskNo, err)
	// 			d.collect(ctx, err)
	// 			continue
	// 		}
	// 	}
	// }); err != nil {
	// 	return err
	// }
	return d.tw.Start()
}

func (d *Storage) Stop(ctx context.Context) error {
	for _, a := range d.adapter {
		if err := a.Close(ctx); err != nil {
			return err
		}
	}
	d.tw.Stop()
	return nil
}

type TaskEntity struct {
	Id           int64             `db:"id"`
	TaskNo       int64             `db:"task_no"`
	Payload      *callback.Payload `db:"payload"`
	DelayTime    int64             `db:"delay_time"`
	Timeout      int64             `db:"timeout"`
	Backoff      *JSONSliceInt64   `db:"backoff"` // JSON array
	CronExpr     string            `db:"cron_expr"`
	Status       int               `db:"status"` // 0待执行 1执行中 2成功 3失败
	NextRunAt    time.Time         `db:"next_run_at"`
	RunTimeoutAt time.Time         `db:"run_timeout_at"`
	FailCount    int               `db:"fail_count"`
	LastRetryAt  *time.Time        `db:"last_retry_at"`
	LockedBy     int64             `db:"locked_by"`
	FailMsgs     *FailMsgs         `db:"fail_msgs"`
	Extra        *Extra            `db:"extra"`
	CreatedAt    time.Time         `db:"created_at"`
	UpdatedAt    time.Time         `db:"updated_at"`
}

func (t *TaskEntity) TraceId() string {
	if t.Extra == nil {
		return ""
	}
	return t.Extra.TraceId
}

func (t *TaskEntity) WithFailMsg(fm *FailMsg) *TaskEntity {
	if t.FailMsgs == nil {
		t.FailMsgs = &FailMsgs{}
	}
	t.FailMsgs.Append(fm)
	return t
}

type JSONSliceInt64 []int64

func (j JSONSliceInt64) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONSliceInt64) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(b, j)
}

type FailMsg struct {
	Resp string `json:"resp,omitempty"`
	Err  string `json:"err,omitempty"`
}
type FailMsgs []*FailMsg

func (j FailMsgs) Value() (driver.Value, error) {
	return json.Marshal(j)
}
func (j *FailMsgs) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(b, j)
}

func (j *FailMsgs) Append(fm *FailMsg) *FailMsgs {
	*j = append(*j, fm)
	return j
}

type Extra struct {
	TraceId string `json:"trace_id"`
}

func (j Extra) Value() (driver.Value, error) {
	return json.Marshal(j)
}
func (j *Extra) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(b, j)
}

func (d *Storage) Add(ctx context.Context, opts ...Option) (int64, error) {
	o := &options{
		delayTime: 5,
		timeout:   3,
		backoff:   []int64{5, 10, 30},
		payload: &callback.Payload{
			Schema: "fmt",
		},
	}
	for _, opt := range opts {
		opt(o)
	}
	taskNo := d.sn.Generate().Int64()
	now := time.Now()

	nextRun := now.Add(time.Duration(o.delayTime) * time.Second)
	runTimeout := nextRun.Add(time.Duration(o.timeout) * time.Second)
	task := &TaskEntity{
		TaskNo:    taskNo,
		Payload:   o.payload,
		DelayTime: o.delayTime,
		Timeout:   o.timeout,
		Backoff: func() *JSONSliceInt64 {
			js := JSONSliceInt64(o.backoff)
			return &js
		}(),
		CronExpr:     o.cron,
		Status:       0,
		NextRunAt:    nextRun,
		RunTimeoutAt: runTimeout,
		FailCount:    -1,
		LockedBy:     int64(d.cfg.Node),
		Extra: &Extra{
			TraceId: trace.Get(ctx),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	d.lg.Info(ctx, "Create Task", "task_no", task.TaskNo, "delay_time", task.DelayTime)
	flag := false
	if time.Duration(o.delayTime)*time.Second <= d.cfg.FastPathTime {
		flag = true
		task.Status = 1
		task.FailCount = 0
	}
	query := `
		INSERT INTO task_queue
        (task_no, payload, delay_time, timeout, backoff, cron_expr, status, next_run_at, run_timeout_at, fail_count, locked_by, extra, created_at, updated_at)
        VALUES
        (:task_no,:payload,:delay_time,:timeout,:backoff,:cron_expr,:status,:next_run_at,:run_timeout_at,:fail_count,:locked_by,:extra,:created_at,:updated_at)
    `
	_, err := d.db.NamedExecContext(ctx, query, task)
	if err != nil {
		return 0, err
	}
	if flag {
		if err = d.Submit(trace.Set(context.Background(), trace.Get(ctx)), task, -1); err != nil {
			return 0, err
		}
	}
	return task.TaskNo, nil
}

func (d *Storage) FetchPendingTasks(ctx context.Context, maxCount int, t time.Duration) ([]*TaskEntity, error) {
	now := time.Now().Add(t)
	var tasks []*TaskEntity
	err := d.db.SelectContext(ctx, &tasks, `
        SELECT * FROM task_queue
        WHERE status=0 AND next_run_at <= ? AND locked_by > ? AND locked_by <= ?
        ORDER BY next_run_at ASC 
        LIMIT ?  FOR UPDATE SKIP LOCKED
    `, now, d.ns[0], d.ns[1], maxCount)
	return tasks, err
}

func (d *Storage) FetchTimeoutTasks(ctx context.Context, maxCount int) ([]*TaskEntity, error) {
	now := time.Now()
	var tasks []*TaskEntity
	err := d.db.SelectContext(ctx, &tasks, `
        SELECT * FROM task_queue
        WHERE status=1 AND run_timeout_at <= ? AND locked_by > ? AND locked_by <= ?
        ORDER BY run_timeout_at ASC
        LIMIT ? FOR UPDATE SKIP LOCKED
    `, now, d.ns[0], d.ns[1], maxCount)
	return tasks, err
}

func (d *Storage) Execute(ctx context.Context, task *TaskEntity) (err error) {
	var resp string
	failCount := task.FailCount
	delayTime := d.GetDelayTime(task)
	d.lg.Info(ctx, "Executing Start", "task_no", fmt.Sprintf("%d-%d", task.TaskNo, failCount), "delay_time", delayTime)
	defer func() {
		d.lg.Info(ctx, "Executing End", "task_no", fmt.Sprintf("%d-%d", task.TaskNo, failCount), "delay_time", delayTime, "resp", resp, "err", err)
	}()
	rCtx, cancelFunc := context.WithDeadline(trace.Set(context.Background(), trace.Get(ctx)), task.RunTimeoutAt)
	defer cancelFunc()
	adapter, ok := d.adapter[strings.ToUpper(task.Payload.Schema)]
	if !ok {
		return d.Failure(ctx, task.WithFailMsg(&FailMsg{
			Resp: "",
			Err:  "adapter not found for schema " + task.Payload.Schema,
		}))
	}
	if task.Payload.Data == nil {
		task.Payload.Data = make(map[string]interface{})
	}
	task.Payload.Data["original"] = map[string]interface{}{"msg_no": task.TaskNo, "trace_id": trace.Get(ctx)}
	resp, err = adapter.Request(rCtx, task.Payload)
	if err == nil && resp == "SUCCESS" {
		return d.Success(ctx, task)
	}
	return d.Failure(ctx, task.WithFailMsg(&FailMsg{
		Resp: resp,
		Err: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	}))
}

func (d *Storage) GetDelayTime(task *TaskEntity) int64 {
	tdd := task.DelayTime
	if task.FailCount > 0 {
		tdd = (*task.Backoff)[task.FailCount-1]
	}
	return tdd
}

func (d *Storage) Success(ctx context.Context, task *TaskEntity) error {
	d.lg.Info(ctx, "Executing Success", "task_no", fmt.Sprintf("%d-%d", task.TaskNo, task.FailCount), "delay_time", d.GetDelayTime(task), "task", task)
	_, err := d.db.Exec(`
        UPDATE task_queue
        SET status=2, updated_at = ?
        WHERE task_no=? AND status=1
    `, time.Now(), task.TaskNo)
	return err
}

func (d *Storage) Failure(ctx context.Context, task *TaskEntity) error {
	d.lg.Error(ctx, "Executing Failed", "task_no", fmt.Sprintf("%d-%d", task.TaskNo, task.FailCount), "delay_time", d.GetDelayTime(task), "task", task)
	now := time.Now()

	if task.FailCount < len(*task.Backoff) {
		// 下次重试时间
		task.DelayTime = (*task.Backoff)[task.FailCount]
		task.NextRunAt = now.Add(time.Duration(task.DelayTime) * time.Second)
		task.RunTimeoutAt = task.NextRunAt.Add(time.Duration(task.Timeout) * time.Second)
		task.LastRetryAt = &now
		if time.Duration(task.DelayTime)*time.Second <= d.cfg.FastPathTime {
			task.FailCount++
			if err := d.Submit(ctx, task, 1); err != nil {
				return err
			}
			return nil
		}

		_, err := d.db.ExecContext(ctx, `
            UPDATE task_queue
            SET status=0, fail_count=?, fail_msgs=?, next_run_at=?, run_timeout_at=?, last_retry_at=?, updated_at=?
            WHERE task_no=? AND status=1
        `, task.FailCount, task.FailMsgs, task.NextRunAt, task.RunTimeoutAt, now, now, task.TaskNo)
		return err
	}

	// 达到最大重试次数，标记失败
	_, err := d.db.ExecContext(ctx, `
        UPDATE task_queue
        SET status=3, fail_msgs = ?, updated_at=?
        WHERE task_no=?
    `, task.FailMsgs, now, task.TaskNo)
	return err
}

func (d *Storage) Submit(ctx context.Context, task *TaskEntity, status int) error {
	if task.FailCount >= len(*task.Backoff) {
		return d.Failure(ctx, task)
	}
	now := time.Now()
	if status > -1 {
		lastRetryAt := &now
		if task.FailCount == 0 {
			lastRetryAt = nil
		}
		_, err := d.db.ExecContext(ctx, `
            UPDATE task_queue
            SET status=1, fail_count=?, fail_msgs=?, next_run_at=?, run_timeout_at=?, last_retry_at=?, updated_at=?
            WHERE task_no=? AND status=?
        `, task.FailCount, task.FailMsgs, task.NextRunAt, task.RunTimeoutAt, lastRetryAt, now, task.TaskNo, status)
		if err != nil {
			return err
		}

	}
	delayTime := task.NextRunAt.Sub(now)
	d.lg.Info(ctx, "Join TW", "task_no", fmt.Sprintf("%d-%d", task.TaskNo, task.FailCount), "delay_time", fmt.Sprintf("%d(%d)", func() int64 {
		dd := int64(delayTime / time.Second)
		tdd := d.GetDelayTime(task)
		if dd < tdd {
			dd = tdd
		}
		return dd
	}(), delayTime/time.Second))
	if task.DelayTime != 0 {
		if err := d.AfterFunc(ctx, delayTime, func() {
			if err := d.Execute(ctx, task); err != nil {
				err = fmt.Errorf("execute after task %d: %w", task.TaskNo, err)
				d.collect(ctx, err)
				return
			}
		}); err != nil {
			return err
		}
	}

	if len(task.CronExpr) != 0 {
		duration, err := CronToDuration(task.CronExpr)
		if err != nil {
			return err
		}
		if _, err = d.tw.ScheduleFunc(&timingwheel.EveryScheduler{Interval: duration}, func() {
			if err = d.Execute(ctx, task); err != nil {
				err = fmt.Errorf("execute schedule task %d: %w", task.TaskNo, err)
				d.collect(ctx, err)
				return
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

func (d *Storage) AfterFunc(ctx context.Context, td time.Duration, f func()) (err error) {
	_, err = d.tw.AfterFunc(td, func() {
		defer func() {
			if rev := recover(); rev != nil {
				d.lg.Error(ctx, "coroutine panic", "rev", rev, "stack", string(debug.Stack()))
			}
		}()
		f()
	})
	return
}

func (d *Storage) ScheduleFunc(interval time.Duration, f func(ctx context.Context)) (err error) {
	_, err = d.tw.ScheduleFunc(&timingwheel.EveryScheduler{Interval: interval}, func() {
		f(trace.Append(context.Background(), trace.GenerateTraceID()))
	})
	return
}
