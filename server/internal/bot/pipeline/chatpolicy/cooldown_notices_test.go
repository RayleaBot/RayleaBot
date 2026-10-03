package chatpolicy

import (
	"strconv"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func TestCooldownNoticePeriodFollowsRejectingLimitWindow(t *testing.T) {
	notices := newCooldownNotices(config.RateLimit{Count: 1, Window: time.Minute}, config.RateLimit{Count: 1, Window: time.Hour})
	start := time.Unix(1_700_000_000, 0)

	if _, ok := notices.claim("user", errorcodes.PlatformUserRateLimited, start); !ok {
		t.Fatal("first user notice not claimed")
	}
	if _, ok := notices.claim("user", errorcodes.PlatformUserRateLimited, start.Add(59*time.Second)); ok {
		t.Fatal("second notice claimed inside the user window")
	}
	if _, ok := notices.claim("user", errorcodes.PlatformUserRateLimited, start.Add(time.Minute)); !ok {
		t.Fatal("notice not claimed after the user window")
	}

	if _, ok := notices.claim("group", errorcodes.PlatformRateLimited, start); !ok {
		t.Fatal("first group notice not claimed")
	}
	if _, ok := notices.claim("group", errorcodes.PlatformRateLimited, start.Add(30*time.Minute)); ok {
		t.Fatal("group notice claimed inside the group window")
	}
}

func TestCooldownNoticeReleaseKeepsNewerPeriod(t *testing.T) {
	notices := newCooldownNotices(config.RateLimit{Count: 1, Window: time.Minute}, config.RateLimit{Count: 1, Window: time.Minute})
	start := time.Unix(1_700_000_000, 0)

	stale, _ := notices.claim("user", errorcodes.PlatformUserRateLimited, start)
	if _, ok := notices.claim("user", errorcodes.PlatformUserRateLimited, start.Add(time.Minute)); !ok {
		t.Fatal("notice not claimed after the window")
	}
	notices.release("user", stale)
	if _, ok := notices.claim("user", errorcodes.PlatformUserRateLimited, start.Add(time.Minute+time.Second)); ok {
		t.Fatal("releasing an expired claim removed the current period")
	}
}

func TestCooldownNoticeSweepDropsExpiredPeriods(t *testing.T) {
	notices := newCooldownNotices(config.RateLimit{Count: 1, Window: time.Minute}, config.RateLimit{Count: 1, Window: time.Minute})
	start := time.Unix(1_700_000_000, 0)
	for index := range minCooldownNoticeSweep {
		notices.claim(strconv.Itoa(index), errorcodes.PlatformUserRateLimited, start)
	}
	notices.claim("late", errorcodes.PlatformUserRateLimited, start.Add(time.Hour))
	if len(notices.until) != 1 {
		t.Fatalf("records after sweep = %d, want only the live period", len(notices.until))
	}
}
