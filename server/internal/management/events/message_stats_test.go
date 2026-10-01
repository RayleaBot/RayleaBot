package events

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/messagestats"
)

func TestMessageStatsSlowSubscriberKeepsLatestIsolatedNotice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := NewMessageStatsService()
		slow, unsubscribeSlow := service.Subscribe(1)
		defer unsubscribeSlow()
		fast, unsubscribeFast := service.Subscribe(1)
		defer unsubscribeFast()
		change := messagestats.Change{ChangedAt: time.Now(), AdapterIDs: []string{"first"}}
		for _, id := range []string{"first", "second", "latest"} {
			change.ChangedAt = change.ChangedAt.Add(2 * time.Second)
			change.AdapterIDs[0] = id
			service.PublishChanged(change)
			got := (<-fast).Data.(MessageStatsPayload).MessageStats
			if got.AdapterIDs[0] != id {
				t.Fatalf("fast subscriber received %v, want %s", got, id)
			}
			got.AdapterIDs[0] = "mutated-by-subscriber"
		}
		change.AdapterIDs[0] = "mutated-by-publisher"
		got := (<-slow).Data.(MessageStatsPayload).MessageStats
		if got.AdapterIDs[0] != "latest" || got.ChangedAt != change.ChangedAt.UTC().Format(time.RFC3339Nano) {
			t.Fatalf("slow subscriber did not keep an isolated latest notice: %+v", got)
		}
	})
}
