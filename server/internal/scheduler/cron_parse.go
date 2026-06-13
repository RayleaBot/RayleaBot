package scheduler

import (
	"fmt"
	"strconv"
	"strings"
)

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
