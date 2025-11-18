package storage

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// CronToDuration converts a standard 6-field cron expression into a time.Duration.
// Supported formats:
//
//	"*/10 * * * * *" => 10s
//	"0 */5 * * * *"  => 5m
//	"0 0 */1 * * *"  => 1h
//	"0 0 0 */1 * *"  => 24h
func CronToDuration(expr string) (time.Duration, error) {
	parts := strings.Fields(expr)
	if len(parts) != 6 {
		return 0, errors.New("unsupported cron format: must have 6 fields (sec min hour day month week)")
	}

	sec, min, hour, day := parts[0], parts[1], parts[2], parts[3]

	// Case 1: */X seconds
	if strings.HasPrefix(sec, "*/") {
		n, _ := strconv.Atoi(strings.TrimPrefix(sec, "*/"))
		return time.Duration(n) * time.Second, nil
	}

	// Case 2: */X minutes
	if strings.HasPrefix(min, "*/") {
		n, _ := strconv.Atoi(strings.TrimPrefix(min, "*/"))
		return time.Duration(n) * time.Minute, nil
	}

	// Case 3: */X hours
	if strings.HasPrefix(hour, "*/") {
		n, _ := strconv.Atoi(strings.TrimPrefix(hour, "*/"))
		return time.Duration(n) * time.Hour, nil
	}

	// Case 4: */X days
	if strings.HasPrefix(day, "*/") {
		n, _ := strconv.Atoi(strings.TrimPrefix(day, "*/"))
		return time.Duration(n*24) * time.Hour, nil
	}

	return 0, errors.New("unsupported cron interval expression")
}
