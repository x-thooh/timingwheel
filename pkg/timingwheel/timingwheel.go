package timingwheel

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/panjf2000/ants"
	"github.com/x-thooh/delay/pkg/timingwheel/bucket"
	"github.com/x-thooh/delay/pkg/timingwheel/queue"
)

// TimingWheel is an implementation of Hierarchical Timing Wheels.
type TimingWheel struct {
	tick      int64 // in milliseconds
	wheelSize int64

	interval    int64 // in milliseconds
	currentTime int64 // in milliseconds
	buckets     []*bucket.Bucket
	queue       *queue.DelayQueue

	// The higher-level overflow wheel.
	//
	// NOTE: This field may be updated and read concurrently, through Add().
	overflowWheel unsafe.Pointer // type: *TimingWheel

	exitC     chan struct{}
	isClosed  bool
	waitGroup waitGroupWrapper

	o    *Options
	pool *ants.Pool
}

// New creates an instance of TimingWheel with the given tick and wheelSize.
func New(tick time.Duration, wheelSize int64, opts ...Option) (*TimingWheel, error) {
	o := &Options{
		poolSize: 1000,
		timeout:  3 * time.Second,
	}
	for _, opt := range opts {
		opt(o)
	}
	tickMs := int64(tick / time.Millisecond)
	if tickMs <= 0 {
		panic(errors.New("tick must be greater than or equal to 1ms"))
	}

	startMs := timeToMs(time.Now().UTC())

	pool, err := ants.NewPool(o.poolSize, o.aos...)
	if err != nil {
		return nil, err
	}

	tw := newTimingWheel(
		tickMs,
		wheelSize,
		startMs,
		queue.New(int(wheelSize)),
	)
	tw.o = o
	tw.pool = pool
	tw.waitGroup = waitGroupWrapper{
		pool: tw.pool,
	}

	return tw, nil
}

// newTimingWheel is an internal helper function that really creates an instance of TimingWheel.
func newTimingWheel(tickMs int64, wheelSize int64, startMs int64, queue *queue.DelayQueue) *TimingWheel {
	buckets := make([]*bucket.Bucket, wheelSize)
	for i := range buckets {
		buckets[i] = bucket.NewBucket()
	}
	return &TimingWheel{
		tick:        tickMs,
		wheelSize:   wheelSize,
		currentTime: truncate(startMs, tickMs),
		interval:    tickMs * wheelSize,
		buckets:     buckets,
		queue:       queue,
		exitC:       make(chan struct{}),
	}
}

// add inserts the timer t into the current timing wheel.
func (tw *TimingWheel) add(t *bucket.Timer) bool {
	currentTime := atomic.LoadInt64(&tw.currentTime)
	if expiration := t.GetExpiration(); expiration < currentTime+tw.tick {
		// Already expired
		return false
	} else if expiration < currentTime+tw.interval {
		// Put it into its own bucket
		virtualID := expiration / tw.tick
		b := tw.buckets[virtualID%tw.wheelSize]
		b.Add(t)

		// Set the bucket expiration time
		if b.SetExpiration(virtualID * tw.tick) {
			// The bucket needs to be enqueued since it was an expired bucket.
			// We only need to enqueue the bucket when its expiration time has changed,
			// i.e. the wheel has advanced and this bucket get reused with a new expiration.
			// Any further calls to set the expiration within the same wheel cycle will
			// pass in the same value and hence return false, thus the bucket with the
			// same expiration will not be enqueued multiple times.
			tw.queue.Offer(b, b.Expiration())
		}

		return true
	} else {
		// Out of the interval. Put it into the overflow wheel
		overflowWheel := atomic.LoadPointer(&tw.overflowWheel)
		if overflowWheel == nil {
			atomic.CompareAndSwapPointer(
				&tw.overflowWheel,
				nil,
				unsafe.Pointer(newTimingWheel(
					tw.interval,
					tw.wheelSize,
					currentTime,
					tw.queue,
				)),
			)
			overflowWheel = atomic.LoadPointer(&tw.overflowWheel)
		}
		return (*TimingWheel)(overflowWheel).add(t)
	}
}

// addOrRun inserts the timer t into the current timing wheel, or run the
// timer's task if it has already expired.
func (tw *TimingWheel) addOrRun(t *bucket.Timer) error {
	if !tw.add(t) {
		// Already expired

		// Like the standard time.AfterFunc (https://golang.org/pkg/time/#AfterFunc),
		// always execute the timer's task in its own goroutine.
		// go t.task()
		if err := tw.waitGroup.Wrap(t.GetTask()); err != nil {
			return err
		}
	}
	return nil
}

func (tw *TimingWheel) advanceClock(expiration int64) {
	currentTime := atomic.LoadInt64(&tw.currentTime)
	if expiration >= currentTime+tw.tick {
		currentTime = truncate(expiration, tw.tick)
		atomic.StoreInt64(&tw.currentTime, currentTime)

		// Try to advance the clock of the overflow wheel if present
		overflowWheel := atomic.LoadPointer(&tw.overflowWheel)
		if overflowWheel != nil {
			(*TimingWheel)(overflowWheel).advanceClock(currentTime)
		}
	}
}

// Start starts the current timing wheel.
func (tw *TimingWheel) Start() error {
	if err := tw.waitGroup.Wrap(func() {
		tw.queue.Poll(tw.exitC, func() int64 {
			return timeToMs(time.Now().UTC())
		})
	}); err != nil {
		return err
	}

	if err := tw.waitGroup.Wrap(func() {
		for {
			select {
			case elem := <-tw.queue.C:
				b := elem.(*bucket.Bucket)
				tw.advanceClock(b.Expiration())
				_ = b.Flush(tw.addOrRun)
			case <-tw.exitC:
				return
			}
		}
	}); err != nil {
		return err
	}

	return nil
}

// Stop stops the current timing wheel.
//
// If there is any timer's task being running in its own goroutine, Stop does
// not wait for the task to complete before returning. If the caller needs to
// know whether the task is completed, it must coordinate with the task explicitly.
func (tw *TimingWheel) Stop() {
	if tw.isClosed {
		return
	}
	close(tw.exitC)
	tw.isClosed = true
	tw.waitGroup.Wait()
	tw.pool.Release()
}

// AfterFunc waits for the duration to elapse and then calls f in its own goroutine.
// It returns a Timer that can be used to cancel the call using its Stop method.
func (tw *TimingWheel) AfterFunc(d time.Duration, f func()) (*bucket.Timer, error) {
	t := bucket.NewTimer(timeToMs(time.Now().UTC().Add(d)), f)
	return t, tw.addOrRun(t)
}

// Scheduler determines the execution plan of a task.
type Scheduler interface {
	// Next returns the next execution time after the given (previous) time.
	// It will return a zero time if no next time is scheduled.
	//
	// All times must be UTC.
	Next(time.Time) time.Time
}

// ScheduleFunc calls f (in its own goroutine) according to the execution
// plan scheduled by s. It returns a Timer that can be used to cancel the
// call using its Stop method.
//
// If the caller want to terminate the execution plan halfway, it must
// stop the timer and ensure that the timer is stopped actually, since in
// the current implementation, there is a gap between the expiring and the
// restarting of the timer. The wait time for ensuring is short since the
// gap is very small.
//
// Internally, ScheduleFunc will ask the first execution time (by calling
// s.Next()) initially, and create a timer if the execution time is non-zero.
// Afterwards, it will ask the next execution time each time f is about to
// be executed, and f will be called at the next execution time if the time
// is non-zero.
func (tw *TimingWheel) ScheduleFunc(s Scheduler, f func()) (t *bucket.Timer, err error) {
	expiration := s.Next(time.Now().UTC())
	if expiration.IsZero() {
		// No time is scheduled, return nil.
		return nil, errors.New("no time is scheduled")
	}

	t = bucket.NewTimer(timeToMs(expiration), func() {
		// Schedule the task to execute at the next time if possible.
		expiration = s.Next(msToTime(t.GetExpiration()))
		if !expiration.IsZero() {
			t.SetExpiration(timeToMs(expiration))
			_ = tw.addOrRun(t)
		}

		// Actually execute the task.
		f()
	})

	return t, tw.addOrRun(t)
}

type waitGroupWrapper struct {
	sync.WaitGroup
	pool *ants.Pool
}

func (w *waitGroupWrapper) Wrap(cb func()) error {
	w.Add(1)
	if w.pool != nil {
		return w.pool.Submit(func() {
			cb()
			w.Done()
		})
	}
	go func() {
		cb()
		w.Done()
	}()
	return nil
}
