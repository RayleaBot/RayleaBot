package scheduler

import (
	"fmt"
	"strconv"
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

// parseField parses a single cron field into a sorted list of allowed values.
func parseField(field string, min, max int) ([]int, error) {
	var result []int
	parts := strings.Split(field, ",")
	for _, part := range parts {
		vals, err := parsePart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, vals...)
	}
	return dedupSort(result, min, max), nil
}

func parsePart(part string, min, max int) ([]int, error) {
	// Handle step: */N or M-N/S
	if idx := strings.Index(part, "/"); idx >= 0 {
		base := part[:idx]
		stepStr := part[idx+1:]
		step, err := strconv.Atoi(stepStr)
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step %q", stepStr)
		}
		start, end := min, max
		if base != "*" {
			s, e, err := parseRange(base, min, max)
			if err != nil {
				return nil, err
			}
			start, end = s, e
		}
		var vals []int
		for v := start; v <= end; v += step {
			vals = append(vals, v)
		}
		return vals, nil
	}

	// Handle wildcard.
	if part == "*" {
		vals := make([]int, 0, max-min+1)
		for v := min; v <= max; v++ {
			vals = append(vals, v)
		}
		return vals, nil
	}

	// Handle range: M-N
	if strings.Contains(part, "-") {
		s, e, err := parseRange(part, min, max)
		if err != nil {
			return nil, err
		}
		vals := make([]int, 0, e-s+1)
		for v := s; v <= e; v++ {
			vals = append(vals, v)
		}
		return vals, nil
	}

	// Single value.
	v, err := strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid value %q", part)
	}
	if v < min || v > max {
		return nil, fmt.Errorf("value %d out of range [%d, %d]", v, min, max)
	}
	return []int{v}, nil
}

func parseRange(s string, min, max int) (int, int, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q", s)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range start %q", parts[0])
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range end %q", parts[1])
	}
	if start < min || end > max || start > end {
		return 0, 0, fmt.Errorf("range %d-%d out of bounds [%d, %d]", start, end, min, max)
	}
	return start, end, nil
}

func toSet(vals []int) map[int]bool {
	m := make(map[int]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

func dedupSort(vals []int, min, max int) []int {
	seen := make(map[int]bool, len(vals))
	result := make([]int, 0, len(vals))
	for _, v := range vals {
		if v >= min && v <= max && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	// Simple insertion sort — field values are small.
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}
