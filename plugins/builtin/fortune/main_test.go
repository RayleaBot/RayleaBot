package main

import (
	"testing"
	"time"
)

func TestDrawDailyIsStableAndUnique(t *testing.T) {
	current := settings{
		Fortunes: []fortune{
			{Name: "大吉", Stars: "★★★★★★★", Sign: "a", Explanation: "a"},
			{Name: "中吉", Stars: "★★★★★☆☆", Sign: "b", Explanation: "b"},
			{Name: "吉凶未定", Stars: "???????", Sign: "c", Explanation: "c"},
		},
		GoodActions: []string{"读书", "散步", "整理"},
		BadActions:  []string{"熬夜", "冲动", "拖延", "读书"},
	}
	first := drawDaily(current, "42", "2026-08-02")
	second := drawDaily(current, "42", "2026-08-02")
	if first.Fortune.Name != second.Fortune.Name || len(first.TodayGood) != 2 || first.TodayGood[0] == first.TodayGood[1] {
		t.Fatalf("unstable draw: %#v %#v", first, second)
	}
	if first.Fortune.Name == "吉凶未定" {
		t.Fatalf("ordinary draw selected reserved fortune: %#v", first)
	}
	for _, bad := range first.TodayBad {
		for _, good := range first.TodayGood {
			if bad == good {
				t.Fatalf("good and bad overlap: %#v", first)
			}
		}
	}
	if first.DrawSourceFingerprint == "" {
		t.Fatal("draw source fingerprint is empty")
	}
}

func TestLocalDayUsesConfiguredTimezone(t *testing.T) {
	if got := localDay(mustTime(t, "2026-08-01T18:00:00Z"), "Asia/Shanghai"); got != "2026-08-02" {
		t.Fatalf("localDay = %q", got)
	}
	if got := localDay(mustTime(t, "2026-08-01T18:00:00Z"), "UTC+08:00"); got != "2026-08-02" {
		t.Fatalf("fixed-offset localDay = %q", got)
	}
	if got := localDay(mustTime(t, "2026-08-02T01:00:00Z"), "America/Los_Angeles"); got != "2026-08-01" {
		t.Fatalf("IANA localDay = %q", got)
	}
}

func TestSpecialDateOverridesOrdinaryDraw(t *testing.T) {
	special := fortune{Name: "吉凶未定", Stars: "???????", Sign: "云深", Explanation: "静候"}
	current := settings{
		Fortunes: []fortune{{Name: "大吉", Stars: "★★★★★★★", Sign: "a", Explanation: "a"}, special},
		SpecialDates: []specialDate{
			{Date: "08-02", FortuneName: "吉凶未定"},
		},
	}
	record := drawDaily(current, "42", "2026-08-02")
	if !record.Special || record.Fortune.Name != "吉凶未定" {
		t.Fatalf("special date not applied: %#v", record)
	}
}

func TestFingerprintChangesWithDrawSettings(t *testing.T) {
	current := settings{Fortunes: []fortune{{Name: "大吉", Stars: "★★★★★★★", Sign: "a", Explanation: "a"}}}
	first := drawSourceFingerprint(current)
	current.GoodActions = []string{"整理计划"}
	if second := drawSourceFingerprint(current); first == second {
		t.Fatal("fingerprint did not change with draw settings")
	}
}

func TestStatsTrackStreaksAndReplacement(t *testing.T) {
	stats := updateStats(emptyStats(), "大吉", "2026-08-01")
	stats = updateStats(stats, "大吉", "2026-08-02")
	if stats.TotalDays != 2 || stats.CurrentStreak != 2 || stats.LongestDajiStreak != 2 || stats.Counts["大吉"] != 2 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	stats = replaceCurrentDayFortune(stats, "大吉", "大凶")
	if stats.Counts["大吉"] != 1 || stats.Counts["大凶"] != 1 || stats.CurrentDajiStreak != 1 || stats.CurrentDaxiongStreak != 1 {
		t.Fatalf("unexpected replacement stats: %#v", stats)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
