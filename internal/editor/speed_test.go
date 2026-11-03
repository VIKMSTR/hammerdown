package editor

import (
	"math"
	"testing"
	"time"
)

func TestSpeed(t *testing.T) {
	var s Speed
	t0 := time.Unix(1000, 0)
	if got := s.PerMinute(t0); got != 0 {
		t.Fatalf("empty: %v", got)
	}

	// Steady 5 keys per second for 10 s → 300 per minute.
	at := t0
	for range 50 {
		s.Record(at)
		at = at.Add(200 * time.Millisecond)
	}
	if got := s.PerMinute(at); math.Abs(got-300) > 15 {
		t.Fatalf("steady: %v, want ~300", got)
	}

	// After a pause longer than the window the reading drops to zero.
	if got := s.PerMinute(at.Add(SpeedWindow)); got != 0 {
		t.Fatalf("after pause: %v", got)
	}
}

func TestSpeedBurstRampsUpQuickly(t *testing.T) {
	var s Speed
	t0 := time.Unix(1000, 0)
	// Two seconds at 5 keys/s: measured from the burst start, not over the
	// whole window, so it already reads ~300 rather than ~120.
	for i := range 10 {
		s.Record(t0.Add(time.Duration(i) * 200 * time.Millisecond))
	}
	if got := s.PerMinute(t0.Add(2 * time.Second)); math.Abs(got-300) > 15 {
		t.Fatalf("burst: %v, want ~300", got)
	}
	// A couple of quick keys do not read as thousands per minute.
	s.Reset()
	s.Record(t0.Add(time.Minute))
	s.Record(t0.Add(time.Minute + 20*time.Millisecond))
	if got := s.PerMinute(t0.Add(time.Minute + 30*time.Millisecond)); got > 120 {
		t.Fatalf("two keys: %v, want <= 120", got)
	}
}
