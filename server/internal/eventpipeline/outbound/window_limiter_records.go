package outbound

import "time"

func pruneWindowRecords(entries []time.Time, now time.Time, window time.Duration) []time.Time {
	if window <= 0 {
		return nil
	}
	cutoff := now.Add(-window)
	index := 0
	for index < len(entries) && entries[index].Before(cutoff) {
		index++
	}
	return append([]time.Time(nil), entries[index:]...)
}
