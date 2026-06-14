package source

import (
	"context"
	"database/sql"
	"time"
)

func (s *Source) setRoomState(ctx context.Context, state roomState) {
	now := s.now()
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
	}
	_, _ = s.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_rooms (uid, room_id, name, face, cover_url, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   room_id = excluded.room_id,
		   name = excluded.name,
		   face = excluded.face,
		   cover_url = excluded.cover_url,
		   live_status = excluded.live_status,
		   live_started_at = excluded.live_started_at,
		   live_event_id = excluded.live_event_id,
		   connection_state = excluded.connection_state,
		   last_event_at = excluded.last_event_at,
		   last_error = excluded.last_error,
		   updated_at = excluded.updated_at`,
		state.UID, state.RoomID, state.Name, state.Face, state.CoverURL, state.LiveStatus, state.LiveStartedAt, state.LiveEventID,
		state.ConnectionState, nullableTimeString(state.LastEventAt), state.LastError, state.UpdatedAt.Format(time.RFC3339),
	)
}
func (s *Source) loadRoomState(ctx context.Context, uid string) roomState {
	var state roomState
	var lastEventAt sql.NullString
	var updatedAt string
	err := s.read.QueryRowContext(ctx,
		`SELECT uid, room_id, name, face, cover_url, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at
		 FROM bilibili_source_rooms WHERE uid = ?`, uid,
	).Scan(&state.UID, &state.RoomID, &state.Name, &state.Face, &state.CoverURL, &state.LiveStatus, &state.LiveStartedAt, &state.LiveEventID,
		&state.ConnectionState, &lastEventAt, &state.LastError, &updatedAt)
	if err != nil {
		return roomState{UID: uid, ConnectionState: StateIdle}
	}
	if lastEventAt.Valid {
		state.LastEventAt = parseRFC3339Ptr(lastEventAt.String)
	}
	state.UpdatedAt = parseRFC3339(updatedAt)
	return state
}
func (s *Source) roomConnectionCounts(ctx context.Context, watchedUIDs map[string]bool) (int, int) {
	if len(watchedUIDs) == 0 {
		return 0, 0
	}
	rows, err := s.read.QueryContext(ctx, `SELECT uid, connection_state FROM bilibili_source_rooms`)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	connected := 0
	failed := 0
	for rows.Next() {
		var uid string
		var state string
		if rows.Scan(&uid, &state) != nil {
			continue
		}
		if !watchedUIDs[uid] {
			continue
		}
		switch state {
		case StateConnected:
			connected++
		case StateFailed, StateDegraded:
			failed++
		}
	}
	return connected, failed
}
func (s *Source) markSeen(ctx context.Context, key, uid, eventType, sourceID string) bool {
	if key == "" {
		return false
	}
	result, err := s.write.ExecContext(ctx,
		`INSERT OR IGNORE INTO bilibili_source_seen (event_key, uid, event_type, source_id, observed_at)
		 VALUES (?, ?, ?, ?, ?)`,
		key, uid, eventType, sourceID, s.now().Format(time.RFC3339),
	)
	if err != nil {
		return false
	}
	rows, err := result.RowsAffected()
	return err == nil && rows > 0
}
