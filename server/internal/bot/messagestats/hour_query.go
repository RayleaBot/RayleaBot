package messagestats

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

// sqlc materializes one growing slice. Read the same range into bounded chunks
// so a large history does not repeatedly copy rows as its result grows.
func (s *Service) queryHours(ctx context.Context, from, through int64) ([][]sqlcgen.MessageStatsHour, error) {
	rows, err := s.store.Read.QueryContext(ctx,
		`SELECT hour_start, adapter_id, received, sent FROM message_stats_hours WHERE hour_start >= ? AND hour_start < ?`,
		from, through)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var chunks [][]sqlcgen.MessageStatsHour
	var chunk []sqlcgen.MessageStatsHour
	var row sqlcgen.MessageStatsHour
	destination := []any{&row.HourStart, &row.AdapterID, &row.Received, &row.Sent}
	used, nextSize := 0, 64
	for rows.Next() {
		if err := rows.Scan(destination...); err != nil {
			return nil, err
		}
		if used == len(chunk) {
			chunk = make([]sqlcgen.MessageStatsHour, nextSize)
			chunks = append(chunks, chunk)
			used = 0
			nextSize = min(2*nextSize, 1024)
		}
		chunk[used] = row
		used++
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(chunks) > 0 {
		chunks[len(chunks)-1] = chunk[:used]
	}
	return chunks, nil
}
