package messagestats

import (
	"errors"
	"time"
)

var ErrInvalidRequest = errors.New("invalid message statistics range")

type Query struct{ StartAt, EndAt, Granularity string }

type alignedRange struct {
	start, end  time.Time
	granularity string
	buckets     []time.Time
}

func floor(t time.Time, granularity string, location *time.Location) time.Time {
	if granularity == "hour" {
		return t.UTC().Truncate(time.Hour)
	}
	local := t.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).UTC()
	date := func(at time.Time) int { y, m, d := at.In(location).Date(); return y*10000 + int(m)*100 + d }
	target := date(t)
	if date(start) == target && date(start.Add(-time.Nanosecond)) != target {
		return start
	}
	// Some IANA transitions skip or repeat midnight itself. Find the first
	// instant of the local date rather than accepting Date's normalization.
	low, high := t.Add(-48*time.Hour), t
	for high.Sub(low) > time.Nanosecond {
		middle := low.Add(high.Sub(low) / 2)
		if date(middle) < target {
			low = middle
		} else {
			high = middle
		}
	}
	return high.UTC()
}

func nextBucket(t time.Time, granularity string, location *time.Location) time.Time {
	if granularity == "hour" {
		return t.Add(time.Hour)
	}
	// Calendar arithmetic preserves 23/25-hour days and fractional offsets.
	local := t.In(location)
	next := floor(time.Date(local.Year(), local.Month(), local.Day()+1, 12, 0, 0, 0, location), granularity, location)
	if !next.After(t) {
		next = floor(t.Add(36*time.Hour), granularity, location)
	}
	return next
}

func align(query Query, location *time.Location) (alignedRange, error) {
	start, err := time.Parse(time.RFC3339Nano, query.StartAt)
	if err != nil {
		return alignedRange{}, ErrInvalidRequest
	}
	end, err := time.Parse(time.RFC3339Nano, query.EndAt)
	if err != nil || !end.After(start) || (query.Granularity != "hour" && query.Granularity != "day") {
		return alignedRange{}, ErrInvalidRequest
	}
	r := alignedRange{start: floor(start, query.Granularity, location), end: floor(end, query.Granularity, location), granularity: query.Granularity}
	if r.end.Before(end) {
		r.end = nextBucket(r.end, r.granularity, location)
	}
	limit := 744
	if r.granularity == "day" {
		limit = 732
	}
	for bucket := r.start; bucket.Before(r.end); bucket = nextBucket(bucket, r.granularity, location) {
		if len(r.buckets) == limit {
			return alignedRange{}, ErrInvalidRequest
		}
		r.buckets = append(r.buckets, bucket)
	}
	return r, nil
}
