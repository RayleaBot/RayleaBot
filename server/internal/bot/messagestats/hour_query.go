package messagestats

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

type hourCounts struct {
	start  int64
	counts Counts
}

type adapterHours struct {
	adapter string
	chunks  [][]hourCounts
}

// Keep one adapter ID per group and grow chunks without copying earlier rows.
// Small initial chunks also bound unused capacity for sparse adapter histories.
func (s *Service) queryHours(ctx context.Context, from, through int64) ([]adapterHours, error) {
	rows, err := s.store.Read.QueryContext(ctx,
		`SELECT hour_start, adapter_id, received, sent FROM message_stats_hours WHERE hour_start >= ? AND hour_start < ?`,
		from, through)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var groups []adapterHours
	byAdapter := make(map[string]int)
	var row sqlcgen.MessageStatsHour
	destination := []any{&row.HourStart, &row.AdapterID, &row.Received, &row.Sent}
	for rows.Next() {
		if err := rows.Scan(destination...); err != nil {
			return nil, err
		}
		index, ok := byAdapter[row.AdapterID]
		if !ok {
			index = len(groups)
			byAdapter[row.AdapterID] = index
			groups = append(groups, adapterHours{adapter: row.AdapterID})
		}
		group := &groups[index]
		last := len(group.chunks) - 1
		if last < 0 || len(group.chunks[last]) == cap(group.chunks[last]) {
			nextSize := 1
			if last >= 0 {
				nextSize = min(2*cap(group.chunks[last]), 1024)
			}
			group.chunks = append(group.chunks, make([]hourCounts, 0, nextSize))
			last++
		}
		group.chunks[last] = append(group.chunks[last], hourCounts{row.HourStart, Counts{row.Received, row.Sent}})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}
