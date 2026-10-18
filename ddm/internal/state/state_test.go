package state

import (
	"strings"
	"testing"
	"time"
)

func TestWhy(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	built := Build{Commit: "aaa", Stamp: "s", At: ago(30 * time.Minute)}
	failed := built
	failed.Error = "boom"

	cases := []struct {
		name      string
		b         Build
		head      string
		stamp     string
		published bool
		cooldown  time.Duration
		run       bool
		why       string
	}{
		{"never built", Build{}, "aaa", "s", false, 0, true, "never built"},
		{"up to date", built, "aaa", "s", true, 0, false, "up to date"},
		{"new commit, no cooldown", built, "bbb", "s", true, 0, true, "new commit"},
		{"new commit, cooling down", built, "bbb", "s", true, time.Hour, false, "waits for the cooldown"},
		{"new commit, cooled down", built, "bbb", "s", true, 20 * time.Minute, true, "new commit"},
		{"spec changed skips the cooldown", built, "aaa", "t", true, 24 * time.Hour, true, "spec or ddm changed"},
		{"publish gone", built, "aaa", "s", false, time.Hour, true, "not published"},
		{"failed recently", failed, "aaa", "s", true, 0, false, "retrying after"},
		{"failed recently, new commit", failed, "bbb", "s", true, 0, true, "new commit"},
		{"failed long ago", Build{Commit: "aaa", Stamp: "s", At: ago(2 * time.Hour), Error: "boom"}, "aaa", "s", true, 0, true, "retrying"},
	}
	for _, c := range cases {
		run, why := c.b.Why(c.head, c.stamp, c.published, c.cooldown, now)
		if run != c.run || !strings.Contains(why, c.why) {
			t.Errorf("%s: got (%v, %q), want (%v, …%q…)", c.name, run, why, c.run, c.why)
		}
	}
}

func TestBusy(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	st := &State{Runs: []Run{
		{now.Add(-30 * time.Hour), now.Add(-23 * time.Hour)}, // 1h inside the window
		{now.Add(-2 * time.Hour), now.Add(-time.Hour)},       // 1h
	}}
	if got := st.Busy(now, 24*time.Hour); got < 0.083 || got > 0.084 {
		t.Errorf("busy: got %.4f, want 2/24", got)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	st, err := Load(dir)
	if err != nil || len(st.Builds) != 0 {
		t.Fatalf("empty load: %v %v", st, err)
	}
	st.Builds[Key("public", "forest")] = Build{Commit: "aaa", Stamp: "s", At: now}
	st.Runs = []Run{{now.Add(-8 * 24 * time.Hour), now.Add(-8 * 24 * time.Hour)}, {now.Add(-time.Minute), now}}
	if err := st.Save(dir, now); err != nil {
		t.Fatal(err)
	}
	again, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.Builds["public/forest"].Commit != "aaa" || len(again.Runs) != 1 {
		t.Errorf("round trip: %+v", again)
	}
}
