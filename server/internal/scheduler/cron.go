package scheduler

import (
	"fmt"
	"strings"
	"time"
)

// nextCronTime computes the next fire time after `after` for a standard
// 5-field cron expression: minute hour day-of-month month day-of-week.
// Supports: literal values, `*`, ranges (`1-5`), steps (`*/15`), and lists (`1,15,30`).
func nextCronTime(expr string, after time.Time, loc *time.Location) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("minute field: %w", err)
	}
	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("hour field: %w", err)
	}
	doms, err := parseField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("day-of-month field: %w", err)
	}
	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("month field: %w", err)
	}
	dows, err := parseField(fields[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("day-of-week field: %w", err)
	}

	monthSet := toSet(months)
	domSet := toSet(doms)
	dowSet := toSet(dows)
	hourSet := toSet(hours)
	minuteSet := toSet(minutes)

	// Start searching from the next minute after `after`.
	t := after.In(loc).Truncate(time.Minute).Add(time.Minute)

	// Search up to ~4 years to find a match.
	limit := t.Add(4 * 365 * 24 * time.Hour)
	for t.Before(limit) {
		if !monthSet[int(t.Month())] {
			// Advance to next month.
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !domSet[t.Day()] || !dowSet[int(t.Weekday())] {
			t = t.AddDate(0, 0, 1)
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			continue
		}
		if !hourSet[t.Hour()] {
			t = t.Add(time.Hour)
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
			continue
		}
		if !minuteSet[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("no matching time found within 4 years for %q", expr)
}
