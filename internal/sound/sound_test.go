package sound

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"
)

type fakeOut struct{ plays [][]byte }

func (f *fakeOut) play(pcm []byte) { f.plays = append(f.plays, pcm) }

// newTest returns a player wired to a fake output and a controllable clock.
func newTest(t *testing.T) (*Player, *fakeOut, *time.Time, *int) {
	t.Helper()
	p := New()
	out := &fakeOut{}
	opens := 0
	p.open = func() (output, error) { opens++; return out, nil }
	clock := time.Unix(1000, 0)
	p.now = func() time.Time { return clock }
	return p, out, &clock, &opens
}

func TestAssetsLoad(t *testing.T) {
	banks, err := loadBanks()
	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range Profiles[1:] {
		for _, k := range []Kind{Key, Space, Return, Delete} {
			v, ok := banks[pr][k]
			if !ok || len(v.clips) == 0 {
				t.Fatalf("%s/%d: no clips", pr, k)
			}
			for _, c := range v.clips {
				// Keep feedback short: nothing over a second, keys well under.
				if d := time.Duration(len(c)) * time.Second / SampleRate; d > time.Second || (k == Key && d > 300*time.Millisecond) {
					t.Errorf("%s/%d: clip is %v long", pr, k, d)
				}
			}
		}
	}
}

func TestOffIsSilentAndDoesNotOpenDevice(t *testing.T) {
	p, out, _, opens := newTest(t)
	p.Play(Key)
	p.Preview()
	if *opens != 0 || len(out.plays) != 0 {
		t.Fatalf("opens=%d plays=%d, want 0/0", *opens, len(out.plays))
	}
}

func TestPlayRendersWithinRange(t *testing.T) {
	p, out, clock, opens := newTest(t)
	p.SetProfile(Typewriter)
	p.SetVolume(1)
	for i, k := range []Kind{Key, Space, Return, Delete} {
		*clock = clock.Add(time.Second)
		p.Play(k)
		if len(out.plays) != i+1 {
			t.Fatalf("kind %d not played", k)
		}
	}
	if *opens != 1 {
		t.Fatalf("device opened %d times, want 1", *opens)
	}
	for _, pcm := range out.plays {
		if len(pcm) == 0 || len(pcm)%4 != 0 {
			t.Fatalf("bad pcm length %d", len(pcm))
		}
		peak := float32(0)
		for i := 0; i < len(pcm); i += 4 {
			s := math.Float32frombits(binary.LittleEndian.Uint32(pcm[i:]))
			peak = max(peak, float32(math.Abs(float64(s))))
		}
		if peak == 0 || peak > 1 {
			t.Fatalf("peak %v out of (0, 1]", peak)
		}
	}
}

func TestRateLimitAndVolumeZero(t *testing.T) {
	p, out, clock, _ := newTest(t)
	p.SetProfile(Bubble)
	p.Play(Key)
	p.Play(Key) // same instant: key repeat, dropped
	*clock = clock.Add(minInterval)
	p.Play(Key)
	if len(out.plays) != 2 {
		t.Fatalf("plays=%d, want 2", len(out.plays))
	}
	p.SetVolume(0)
	*clock = clock.Add(time.Second)
	p.Play(Key)
	if len(out.plays) != 2 {
		t.Fatal("played at volume 0")
	}
}

func TestVariantsDoNotRepeat(t *testing.T) {
	p, _, clock, _ := newTest(t)
	p.SetProfile(Bubble)
	prev := -1
	for range 50 {
		*clock = clock.Add(time.Second)
		p.Play(Key)
		if p.last[Key] == prev {
			t.Fatal("same bubble variant twice in a row")
		}
		prev = p.last[Key]
	}
}

func TestOpenErrorReportedOnce(t *testing.T) {
	p := New()
	errs := make(chan error, 4)
	p.OnError = func(err error) { errs <- err }
	p.open = func() (output, error) { return nil, errors.New("no device") }
	p.SetProfile(Buzz)
	p.Play(Key)
	p.SetProfile(Bubble)
	if err := <-errs; err == nil {
		t.Fatal("nil error")
	}
	select {
	case err := <-errs:
		t.Fatalf("error reported twice: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestParseProfile(t *testing.T) {
	for _, pr := range Profiles {
		if ParseProfile(string(pr)) != pr {
			t.Errorf("round trip %q", pr)
		}
	}
	if ParseProfile("") != Off || ParseProfile("kazoo") != Off {
		t.Error("unknown values must map to Off")
	}
}

func TestResample(t *testing.T) {
	src := []float32{0, 1, 2, 3, 4}
	if got := resample(src, 2); len(got) != 3 || got[1] != 2 || got[2] != 4 {
		t.Errorf("ratio 2: %v", got)
	}
	if got := resample(src, 0.5); len(got) != 9 || got[1] != 0.5 || got[8] != 4 {
		t.Errorf("ratio 0.5: %v", got)
	}
}

func TestDecodeWAVRejectsGarbage(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("RIFF0000WAVE"), []byte("hello world, not a wav")} {
		if _, _, err := decodeWAV(b); err == nil {
			t.Errorf("accepted %q", b)
		}
	}
}
