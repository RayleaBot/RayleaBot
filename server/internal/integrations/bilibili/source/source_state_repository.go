package source

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
)

type sourceStateRepository struct {
	read  *sql.DB
	write *sql.DB
	now   func() time.Time
}

func newSourceStateRepository(read, write *sql.DB, now func() time.Time) *sourceStateRepository {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &sourceStateRepository{read: read, write: write, now: now}
}

type sourceRoom struct {
	UID             string
	RoomID          string
	Name            string
	Face            string
	CoverURL        string
	LiveStatus      int
	LiveStartedAt   int64
	LiveEventID     string
	ConnectionState string
	LastEventAt     *time.Time
	LastError       string
	UpdatedAt       time.Time
}

func (r *sourceStateRepository) SetRoom(ctx context.Context, state sourceRoom) {
	if r == nil || r.write == nil {
		return
	}
	now := r.now()
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
	}
	_, _ = r.write.ExecContext(ctx,
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

func (r *sourceStateRepository) LoadRoom(ctx context.Context, uid, idleState string) sourceRoom {
	if r == nil || r.read == nil {
		return sourceRoom{UID: uid, ConnectionState: idleState}
	}
	var state sourceRoom
	var lastEventAt sql.NullString
	var updatedAt string
	err := r.read.QueryRowContext(ctx,
		`SELECT uid, room_id, name, face, cover_url, live_status, live_started_at, live_event_id, connection_state, last_event_at, last_error, updated_at
		 FROM bilibili_source_rooms WHERE uid = ?`, uid,
	).Scan(&state.UID, &state.RoomID, &state.Name, &state.Face, &state.CoverURL, &state.LiveStatus, &state.LiveStartedAt, &state.LiveEventID,
		&state.ConnectionState, &lastEventAt, &state.LastError, &updatedAt)
	if err != nil {
		return sourceRoom{UID: uid, ConnectionState: idleState}
	}
	if lastEventAt.Valid {
		state.LastEventAt = parseRFC3339Ptr(lastEventAt.String)
	}
	state.UpdatedAt = parseRFC3339(updatedAt)
	return state
}

func (r *sourceStateRepository) RoomConnectionCounts(ctx context.Context, watchedUIDs map[string]bool, connectedState string, failedStates map[string]bool) (int, int) {
	if r == nil || r.read == nil || len(watchedUIDs) == 0 {
		return 0, 0
	}
	rows, err := r.read.QueryContext(ctx, `SELECT uid, connection_state FROM bilibili_source_rooms`)
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
		if state == connectedState {
			connected++
			continue
		}
		if failedStates[state] {
			failed++
		}
	}
	return connected, failed
}

func (r *sourceStateRepository) MarkSeen(ctx context.Context, key, uid, eventType, sourceID string) bool {
	if r == nil || r.write == nil || key == "" {
		return false
	}
	result, err := r.write.ExecContext(ctx,
		`INSERT OR IGNORE INTO bilibili_source_seen (event_key, uid, event_type, source_id, observed_at)
		 VALUES (?, ?, ?, ?, ?)`,
		key, uid, eventType, sourceID, r.now().Format(time.RFC3339),
	)
	if err != nil {
		return false
	}
	rows, err := result.RowsAffected()
	return err == nil && rows > 0
}

func (r *sourceStateRepository) HasSeen(ctx context.Context, uid, eventType string) bool {
	if r == nil || r.read == nil {
		return false
	}
	var exists int
	err := r.read.QueryRowContext(ctx,
		`SELECT 1 FROM bilibili_source_seen WHERE uid = ? AND event_type = ? LIMIT 1`,
		uid, eventType,
	).Scan(&exists)
	return err == nil && exists == 1
}

type sourceDynamic struct {
	UID         string
	DynamicID   string
	Service     string
	Title       string
	Summary     string
	URL         string
	Username    string
	AvatarURL   string
	Images      []bilibilimonitoring.Image
	PublishedAt *time.Time
	ObservedAt  time.Time
	UpdatedAt   time.Time
}

func (r *sourceStateRepository) SetDynamic(ctx context.Context, event bilibilimonitoring.Event) {
	if r == nil || r.write == nil || event.UID == "" || event.ID == "" {
		return
	}
	rawImages, err := json.Marshal(event.Images)
	if err != nil {
		rawImages = []byte("[]")
	}
	now := r.now()
	observedAt := now
	publishedAt := int64(0)
	if event.PubTS > 0 {
		publishedAt = event.PubTS
	}
	_, _ = r.write.ExecContext(ctx,
		`INSERT INTO bilibili_source_dynamics (uid, dynamic_id, service, title, summary, url, username, avatar_url, images_json, published_at, observed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   dynamic_id = excluded.dynamic_id,
		   service = excluded.service,
		   title = excluded.title,
		   summary = excluded.summary,
		   url = excluded.url,
		   username = excluded.username,
		   avatar_url = excluded.avatar_url,
		   images_json = excluded.images_json,
		   published_at = excluded.published_at,
		   observed_at = excluded.observed_at,
		   updated_at = excluded.updated_at`,
		event.UID, event.ID, event.Service, event.Title, event.Summary, event.URL, event.Author.Name, event.Author.Avatar,
		string(rawImages), publishedAt, observedAt.Format(time.RFC3339), now.Format(time.RFC3339),
	)
}

func (r *sourceStateRepository) ClearDynamic(ctx context.Context, uid string) {
	if r == nil || r.write == nil {
		return
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return
	}
	_, _ = r.write.ExecContext(ctx, `DELETE FROM bilibili_source_dynamics WHERE uid = ?`, uid)
}

func (r *sourceStateRepository) LoadDynamics(ctx context.Context) map[string]sourceDynamic {
	if r == nil || r.read == nil {
		return map[string]sourceDynamic{}
	}
	rows, err := r.read.QueryContext(ctx,
		`SELECT uid, dynamic_id, service, title, summary, url, username, avatar_url, images_json, published_at, observed_at, updated_at
		 FROM bilibili_source_dynamics`,
	)
	if err != nil {
		return map[string]sourceDynamic{}
	}
	defer rows.Close()
	result := make(map[string]sourceDynamic)
	for rows.Next() {
		var item sourceDynamic
		var rawImages string
		var publishedAt int64
		var observedAt, updatedAt string
		if err := rows.Scan(
			&item.UID,
			&item.DynamicID,
			&item.Service,
			&item.Title,
			&item.Summary,
			&item.URL,
			&item.Username,
			&item.AvatarURL,
			&rawImages,
			&publishedAt,
			&observedAt,
			&updatedAt,
		); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(rawImages), &item.Images)
		if publishedAt > 0 {
			published := time.Unix(publishedAt, 0).UTC()
			item.PublishedAt = &published
		}
		item.ObservedAt = parseRFC3339(observedAt)
		item.UpdatedAt = parseRFC3339(updatedAt)
		result[item.UID] = item
	}
	return result
}

func (item sourceDynamic) MonitorDynamic() *bilibilimonitoring.MonitorDynamic {
	if item.DynamicID == "" {
		return nil
	}
	return &bilibilimonitoring.MonitorDynamic{
		LastID:      item.DynamicID,
		Service:     item.Service,
		Title:       item.Title,
		Summary:     item.Summary,
		URL:         item.URL,
		Images:      item.Images,
		PublishedAt: item.PublishedAt,
		ObservedAt:  item.ObservedAt,
	}
}

func nullableTimeString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func parseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseRFC3339Ptr(value string) *time.Time {
	parsed := parseRFC3339(value)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}
