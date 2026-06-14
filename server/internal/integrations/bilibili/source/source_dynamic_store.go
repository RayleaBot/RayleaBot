package source

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type dynamicSnapshot struct {
	UID         string
	DynamicID   string
	Service     string
	Title       string
	Summary     string
	URL         string
	Username    string
	AvatarURL   string
	Images      []Image
	PublishedAt *time.Time
	ObservedAt  time.Time
	UpdatedAt   time.Time
}

func (s *Source) setDynamicSnapshot(ctx context.Context, event BilibiliEvent) {
	if event.UID == "" || event.ID == "" {
		return
	}
	rawImages, err := json.Marshal(event.Images)
	if err != nil {
		rawImages = []byte("[]")
	}
	now := s.now()
	observedAt := now
	publishedAt := int64(0)
	if event.PubTS > 0 {
		publishedAt = event.PubTS
	}
	_, _ = s.write.ExecContext(ctx,
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

func (s *Source) clearDynamicSnapshot(ctx context.Context, uid string) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return
	}
	_, _ = s.write.ExecContext(ctx, `DELETE FROM bilibili_source_dynamics WHERE uid = ?`, uid)
}

func (s *Source) clearDynamicSnapshots(ctx context.Context, subjects map[string]Subject) {
	for uid := range subjects {
		s.clearDynamicSnapshot(ctx, uid)
	}
}

func (s *Source) loadDynamicSnapshots(ctx context.Context) map[string]dynamicSnapshot {
	rows, err := s.read.QueryContext(ctx,
		`SELECT uid, dynamic_id, service, title, summary, url, username, avatar_url, images_json, published_at, observed_at, updated_at
		 FROM bilibili_source_dynamics`,
	)
	if err != nil {
		return map[string]dynamicSnapshot{}
	}
	defer rows.Close()
	result := make(map[string]dynamicSnapshot)
	for rows.Next() {
		var item dynamicSnapshot
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
