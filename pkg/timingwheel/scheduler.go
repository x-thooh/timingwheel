package timingwheel

import "time"

type EveryScheduler struct {
	Interval time.Duration
}

func (s *EveryScheduler) Next(prev time.Time) time.Time {
	return prev.Add(s.Interval)
}

type BackOffScheduler struct {
	intervals []time.Duration
	current   int
}

func (s *BackOffScheduler) Next(prev time.Time) time.Time {
	if s.current >= len(s.intervals) {
		return time.Time{}
	}
	next := prev.Add(s.intervals[s.current])
	s.current += 1
	return next
}
