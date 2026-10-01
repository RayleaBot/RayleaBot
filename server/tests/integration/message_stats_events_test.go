package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/messagestats"
	"github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/coder/websocket/wsjson"
)

func TestMessageStatsNoticeReachesWebSocketAndRefetchSeesCount(t *testing.T) {
	application, _, _ := newTestAppWithConfigMutation(t, func(map[string]any) {}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()
	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	// Receiving the initial snapshots proves that all live sources are subscribed.
	for range 2 {
		var frame events.Frame
		if err := wsjson.Read(ctx, conn, &frame); err != nil {
			t.Fatal(err)
		}
	}
	changedAt := time.Now().UTC()
	event := testBridgeEvent()
	event.SourceAdapter = "onebot11"
	application.HandleAdapterEvent(ctx, event)
	for {
		var frame struct {
			Channel string `json:"channel"`
			Type    string `json:"type"`
			Data    struct {
				MessageStats *events.MessageStatsChange `json:"message_stats"`
			} `json:"data"`
		}
		if err := wsjson.Read(ctx, conn, &frame); err != nil {
			t.Fatalf("read statistics change notice: %v", err)
		}
		if frame.Data.MessageStats == nil {
			continue
		}
		notice := frame.Data.MessageStats
		at, err := time.Parse(time.RFC3339Nano, notice.ChangedAt)
		if frame.Channel != "events" || frame.Type != "events.received" || err != nil || at.Before(changedAt.Truncate(time.Millisecond)) || !reflect.DeepEqual(notice.AdapterIDs, []string{"onebot11"}) {
			t.Fatalf("unexpected statistics notice: %+v, %v", frame, err)
		}
		break
	}
	start := changedAt.Truncate(time.Hour)
	path := "/api/system/message-stats?start_at=" + start.Format(time.RFC3339) + "&end_at=" + start.Add(2*time.Hour).Format(time.RFC3339) + "&granularity=hour"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = testutil.TestManagementAuthority
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	application.Handler().ServeHTTP(response, req)
	var stats messagestats.Response
	if err := json.Unmarshal(response.Body.Bytes(), &stats); err != nil || response.Code != http.StatusOK || stats.Totals != (messagestats.Counts{Received: 1}) {
		t.Fatalf("notice refetch did not include the counted message: HTTP %d, %s, %v", response.Code, response.Body.String(), err)
	}
}
