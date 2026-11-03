package editor

import "time"

// SpeedWindow is how far back Speed looks when measuring typing speed.
const SpeedWindow = 5 * time.Second

// minSpan keeps the first keystrokes of a burst from reading as absurd
// speeds (two keys 20 ms apart are not 3000 per minute).
const minSpan = time.Second

// Speed measures typing speed in keystrokes per minute over a sliding
// window. A new burst (after a pause longer than the window) is measured
// from its first keystroke, so the reading ramps up quickly instead of
// starting low and creeping up for the whole window.
type Speed struct {
	times      []time.Time // keystrokes inside the window, oldest first
	burstStart time.Time
}

// Record notes one keystroke at time t.
func (s *Speed) Record(t time.Time) {
	s.trim(t)
	if len(s.times) == 0 {
		s.burstStart = t
	}
	s.times = append(s.times, t)
}

// PerMinute returns the keystrokes per minute at time now; 0 once the
// window holds no keystrokes.
func (s *Speed) PerMinute(now time.Time) float64 {
	s.trim(now)
	if len(s.times) == 0 {
		return 0
	}
	span := SpeedWindow
	if d := now.Sub(s.burstStart); d < span {
		span = max(d, minSpan)
	}
	return float64(len(s.times)) * float64(time.Minute) / float64(span)
}

// Reset forgets all keystrokes.
func (s *Speed) Reset() { s.times = s.times[:0] }

func (s *Speed) trim(now time.Time) {
	cut := 0
	for cut < len(s.times) && now.Sub(s.times[cut]) >= SpeedWindow {
		cut++
	}
	if cut > 0 {
		s.times = append(s.times[:0], s.times[cut:]...)
	}
}
