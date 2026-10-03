package messagestats

import (
	"context"
	"database/sql"
	"maps"
	"sort"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

type Counts struct {
	Received int64 `json:"received"`
	Sent     int64 `json:"sent"`
}

func (c *Counts) add(other Counts) { c.Received += other.Received; c.Sent += other.Sent }

type Connection struct {
	AdapterID      string     `json:"adapter_id"`
	Protocol       string     `json:"protocol"`
	Configured     bool       `json:"configured"`
	Received       []int64    `json:"received"`
	Sent           []int64    `json:"sent"`
	Totals         Counts     `json:"totals"`
	Previous       *Counts    `json:"previous,omitempty"`
	LastReceivedAt *time.Time `json:"last_received_at,omitempty"`
}
type Incident struct {
	Kind      string     `json:"kind"`
	AdapterID string     `json:"adapter_id,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}
type Response struct {
	Granularity        string       `json:"granularity"`
	Timezone           string       `json:"timezone"`
	StartAt            time.Time    `json:"start_at"`
	EndAt              time.Time    `json:"end_at"`
	AsOf               time.Time    `json:"as_of"`
	TrackingStartedAt  time.Time    `json:"tracking_started_at"`
	Buckets            []time.Time  `json:"buckets"`
	Totals             Counts       `json:"totals"`
	Previous           *Counts      `json:"previous,omitempty"`
	Connections        []Connection `json:"connections"`
	Incidents          []Incident   `json:"incidents"`
	IncidentsTruncated bool         `json:"incidents_truncated"`
}

func (s *Service) Query(ctx context.Context, query Query) (Response, error) {
	r, err := align(query, s.location)
	if err != nil {
		return Response{}, err
	}
	length := r.end.Sub(r.start)
	previousStart := r.start.Add(-length)
	hasPrevious := !previousStart.Before(s.tracking)
	from := r.start
	if hasPrevious {
		from = previousStart
	}
	snapshot, err := s.querySnapshot(ctx, from, r.start, r.end)
	if err != nil {
		return Response{}, err
	}
	now := snapshot.asOf

	view := Response{Granularity: r.granularity, Timezone: s.location.String(), StartAt: r.start, EndAt: r.end, AsOf: now, TrackingStartedAt: s.tracking,
		Buckets: []time.Time{}, Connections: []Connection{}, Incidents: []Incident{}}
	first := floor(s.tracking, r.granularity, s.location)
	var bucketEnd int64
	for index, b := range r.buckets {
		if !b.Before(first) && b.Before(now) {
			view.Buckets = append(view.Buckets, b)
			bucketEnd = r.end.Unix()
			if index+1 < len(r.buckets) {
				bucketEnd = r.buckets[index+1].Unix()
			}
		}
	}
	previousEnd := minTime(r.end, now).Add(-length)
	previousFrom, previousThrough := ceilUnixSecond(previousStart), ceilUnixSecond(previousEnd)
	if hasPrevious {
		view.Previous = &Counts{}
	}
	connections := make(map[string]*Connection)
	connection := func(id string) *Connection {
		if c, ok := connections[id]; ok {
			return c
		}
		c := &Connection{AdapterID: id, Received: make([]int64, len(view.Buckets)), Sent: make([]int64, len(view.Buckets))}
		if hasPrevious {
			c.Previous = &Counts{}
		}
		connections[id] = c
		return c
	}
	var bucketStart, cachedStart, cachedEnd int64
	if len(view.Buckets) > 0 {
		bucketStart = view.Buckets[0].Unix()
	}
	cachedIndex := 0
	locate := func(start int64) (int, bool) {
		if len(view.Buckets) == 0 || start < bucketStart || start >= bucketEnd {
			return 0, false
		}
		if r.granularity == "hour" {
			return int((start - bucketStart) / 3600), true
		}
		if start < cachedStart || start >= cachedEnd {
			// Reuse the calendar boundaries from alignment, including short/long
			// days. The fallback also handles unordered pending hours.
			cachedIndex = sort.Search(len(view.Buckets), func(index int) bool {
				return view.Buckets[index].Unix() > start
			}) - 1
			cachedStart, cachedEnd = view.Buckets[cachedIndex].Unix(), bucketEnd
			if cachedIndex+1 < len(view.Buckets) {
				cachedEnd = view.Buckets[cachedIndex+1].Unix()
			}
		}
		return cachedIndex, true
	}
	add := func(start int64, adapter string, value Counts) {
		index, inRange := locate(start)
		inPrevious := hasPrevious && start >= previousFrom && start < previousThrough
		if !inRange && !inPrevious {
			return
		}
		c := connection(adapter)
		if inRange {
			c.Received[index] += value.Received
			c.Sent[index] += value.Sent
			c.Totals.add(value)
			view.Totals.add(value)
		}
		if inPrevious {
			c.Previous.add(value)
			view.Previous.add(value)
		}
	}
	for _, chunk := range snapshot.hours {
		for _, row := range chunk {
			add(row.HourStart, row.AdapterID, Counts{row.Received, row.Sent})
		}
	}
	for key, value := range snapshot.pending {
		add(key.start, key.adapter, value)
	}
	cfg := s.config()
	for _, a := range cfg.Adapters {
		c := connection(a.ID)
		c.Protocol, c.Configured = a.Type, true
	}
	setMetadata := func(id, protocol string, last sql.NullInt64) {
		c, ok := connections[id]
		if !ok {
			return
		}
		if !c.Configured {
			c.Protocol = protocol
		}
		if last.Valid {
			at := time.UnixMilli(last.Int64).UTC()
			if c.LastReceivedAt == nil || at.After(*c.LastReceivedAt) {
				c.LastReceivedAt = &at
			}
		}
	}
	for _, meta := range snapshot.metadata {
		setMetadata(meta.AdapterID, meta.Protocol, meta.LastReceivedAtMs)
	}
	for _, meta := range snapshot.pendingMetadata {
		setMetadata(meta.AdapterID, meta.Protocol, meta.LastReceivedAtMs)
	}
	for _, a := range cfg.Adapters {
		if c, ok := connections[a.ID]; ok {
			view.Connections = append(view.Connections, *c)
			delete(connections, a.ID)
		}
	}
	ids := make([]string, 0, len(connections))
	for id := range connections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		view.Connections = append(view.Connections, *connections[id])
	}
	view.Incidents, view.IncidentsTruncated = s.incidents(snapshot.offline, snapshot.stops, snapshot.intervals, r.start, now)
	return view, nil
}

type querySnapshot struct {
	asOf            time.Time
	hours           [][]sqlcgen.MessageStatsHour
	metadata        []sqlcgen.MessageStatsAdapter
	offline         []sqlcgen.MessageStatsOffline
	stops           []sqlcgen.ListMessageStatsStopsRow
	pending         map[hourKey]Counts
	pendingMetadata map[string]sqlcgen.UpsertMessageStatsAdapterParams
	intervals       map[offlineKey]sql.NullInt64
}

func (s *Service) querySnapshot(ctx context.Context, from, start, end time.Time) (querySnapshot, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	now := s.timeNow()
	through := minTime(end, now)
	endSecond := ceilUnixSecond(through)
	q := sqlcgen.New(s.store.Read)
	var snapshot querySnapshot
	var err error
	snapshot.hours, err = s.queryHours(ctx, from.Unix(), endSecond)
	if err != nil {
		return querySnapshot{}, err
	}
	snapshot.metadata, err = q.ListMessageStatsAdapters(ctx)
	if err != nil {
		return querySnapshot{}, err
	}
	snapshot.offline, err = q.ListMessageStatsOffline(ctx, sqlcgen.ListMessageStatsOfflineParams{AsOfMs: now.UnixMilli(), CurrentRunID: s.runID, StartMs: sql.NullInt64{Int64: start.UnixMilli(), Valid: true}})
	if err != nil {
		return querySnapshot{}, err
	}
	snapshot.stops, err = q.ListMessageStatsStops(ctx, sqlcgen.ListMessageStatsStopsParams{AsOfMs: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, StartMs: start.UnixMilli()})
	if err != nil {
		return querySnapshot{}, err
	}
	// The database rows and pending batch must share one flush boundary. Once
	// copied, response assembly no longer needs to delay another query or flush.
	s.mu.Lock()
	snapshot.asOf = s.timeNow()
	snapshot.pending, snapshot.pendingMetadata = maps.Clone(s.pending), maps.Clone(s.metadata)
	snapshot.intervals = s.openOffline(snapshot.asOf)
	s.mu.Unlock()
	return snapshot, nil
}

func (s *Service) incidents(rows []sqlcgen.MessageStatsOffline, stops []sqlcgen.ListMessageStatsStopsRow, intervals map[offlineKey]sql.NullInt64, start, now time.Time) ([]Incident, bool) {
	for _, row := range rows {
		key := offlineKey{row.RunID, row.AdapterID, row.StartedAtMs}
		if _, exists := intervals[key]; !exists {
			intervals[key] = row.EndedAtMs
		}
	}
	items := make([]Incident, 0, len(intervals))
	for key, end := range intervals {
		at := time.UnixMilli(key.start).UTC()
		through := now
		if end.Valid {
			through = time.UnixMilli(end.Int64).UTC()
		}
		if at.Before(s.tracking) || !at.Before(now) || !through.After(start) || through.Sub(at) < 30*time.Second {
			continue
		}
		incident := Incident{Kind: "adapter_offline", AdapterID: key.adapter, StartedAt: at}
		if end.Valid {
			incident.EndedAt = &through
		}
		items = append(items, incident)
	}
	for _, stop := range stops {
		at, end := time.UnixMilli(stop.StartedAtMs.Int64).UTC(), time.UnixMilli(stop.EndedAtMs).UTC()
		if !at.Before(s.tracking) {
			items = append(items, Incident{Kind: "server_stopped", StartedAt: at, EndedAt: &end})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].StartedAt.Before(items[j].StartedAt)
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].AdapterID < items[j].AdapterID
	})
	truncated := len(items) > 1000
	if truncated {
		items = items[len(items)-1000:]
	}
	return items, truncated
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func ceilUnixSecond(instant time.Time) int64 {
	second := instant.Unix()
	if instant.Nanosecond() != 0 {
		second++
	}
	return second
}
