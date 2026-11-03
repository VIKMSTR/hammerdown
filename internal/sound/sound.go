// Package sound plays short feedback sounds while typing.
//
// The samples are cut from CC0 recordings by Joseph Sardin at
// BigSoundBank.com and from CC BY / CC BY-SA keyboard recordings on
// Wikimedia Commons (see assets/CREDITS.md and [Credits]). Each key press picks a variant,
// nudges its pitch and level slightly so fast typing does not sound like a
// machine gun, and mixes it through a single audio output that is opened only
// when a profile other than Off is first used.
package sound

import (
	"embed"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// Profile is a family of sounds, stored in preferences by its string value.
type Profile string

const (
	Off        Profile = "off"
	Buzz       Profile = "buzz"
	Bubble     Profile = "bubble"
	Typewriter Profile = "typewriter"
	SoftClick  Profile = "softclick" // muted mechanical switch, brown/red-like
	Clicky     Profile = "clicky"    // clicky mechanical switch, Cherry MX Blue
)

// Credits is the attribution the sample licences ask for, for an About box.
const Credits = "Typing sounds (excerpted):\n" +
	"Joseph Sardin, BigSoundBank.com — CC0\n" +
	"\"Typing on Keychron V1 Ultra (Red Linear Switch)\" by C40115,\n" +
	"Wikimedia Commons — CC BY 4.0\n" +
	"\"Cherry mx blue sound\" by Xaqaii, Wikimedia Commons — CC BY-SA 4.0"

// Profiles lists the selectable profiles in menu order, from subtle to loud.
var Profiles = []Profile{Off, Buzz, SoftClick, Bubble, Clicky, Typewriter}

// Label is the menu text for a profile.
func (p Profile) Label() string {
	switch p {
	case Buzz:
		return "Soft Buzz"
	case SoftClick:
		return "Soft Click"
	case Bubble:
		return "Bubbles"
	case Clicky:
		return "Clicky"
	case Typewriter:
		return "Typewriter"
	}
	return "Off"
}

// ParseProfile returns the profile for a stored value, Off if unknown.
func ParseProfile(s string) Profile {
	for _, p := range Profiles {
		if string(p) == s {
			return p
		}
	}
	return Off
}

// Kind is the class of key that was pressed.
type Kind int

const (
	Key Kind = iota
	Space
	Return
	Delete
)

// SampleRate is the output rate; the assets are stored at this rate.
const SampleRate = 44100

const (
	maxVoices   = 8                     // concurrent sounds before the oldest is cut
	minInterval = 18 * time.Millisecond // key-repeat faster than this is not voiced
)

//go:embed assets/*.wav
var assets embed.FS

// voice is how one profile sounds for one kind of key.
type voice struct {
	clips  [][]float32
	gain   float32 // level relative to the user volume
	jitter float64 // ± fraction of random pitch change
}

type bank map[Kind]voice

// loadBanks decodes the embedded assets into the sound profiles.
func loadBanks() (map[Profile]bank, error) {
	load := func(names ...string) ([][]float32, error) {
		var out [][]float32
		for _, n := range names {
			b, err := assets.ReadFile("assets/" + n + ".wav")
			if err != nil {
				return nil, err
			}
			s, rate, err := decodeWAV(b)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n, err)
			}
			if rate != SampleRate {
				return nil, fmt.Errorf("%s: sample rate %d, want %d", n, rate, SampleRate)
			}
			out = append(out, s)
		}
		return out, nil
	}
	buzz, err := load("buzz")
	if err != nil {
		return nil, err
	}
	bubbles, err := load("bubble1", "bubble2", "bubble3", "bubble4", "bubble5")
	if err != nil {
		return nil, err
	}
	soft, err := load("click-soft1", "click-soft2", "click-soft3", "click-soft4", "click-soft5")
	if err != nil {
		return nil, err
	}
	blue, err := load("click-blue1", "click-blue2", "click-blue3", "click-blue4", "click-blue5")
	if err != nil {
		return nil, err
	}
	twKey, err := load("typewriter-key")
	if err != nil {
		return nil, err
	}
	twSpace, err := load("typewriter-space")
	if err != nil {
		return nil, err
	}
	twBell, err := load("typewriter-bell")
	if err != nil {
		return nil, err
	}

	return map[Profile]bank{
		Buzz: {
			Key:    voice{clips: buzz, gain: 0.30, jitter: 0.06},
			Space:  voice{clips: buzz, gain: 0.30, jitter: 0.03},
			Return: voice{clips: buzz, gain: 0.34, jitter: 0.02},
			Delete: voice{clips: buzz, gain: 0.22, jitter: 0.03},
		},
		SoftClick: {
			Key:    voice{clips: soft, gain: 0.55, jitter: 0.04},
			Space:  voice{clips: soft, gain: 0.60, jitter: 0.02},
			Return: voice{clips: soft, gain: 0.60, jitter: 0.02},
			Delete: voice{clips: soft, gain: 0.45, jitter: 0.03},
		},
		Clicky: {
			Key:    voice{clips: blue, gain: 0.60, jitter: 0.04},
			Space:  voice{clips: blue, gain: 0.60, jitter: 0.02},
			Return: voice{clips: blue, gain: 0.65, jitter: 0.02},
			Delete: voice{clips: blue, gain: 0.50, jitter: 0.03},
		},
		Bubble: {
			Key:    voice{clips: bubbles, gain: 0.45, jitter: 0.10},
			Space:  voice{clips: bubbles, gain: 0.45, jitter: 0.05},
			Return: voice{clips: bubbles, gain: 0.50, jitter: 0.03},
			Delete: voice{clips: bubbles, gain: 0.35, jitter: 0.05},
		},
		Typewriter: {
			Key:    voice{clips: twKey, gain: 0.90, jitter: 0.05},
			Space:  voice{clips: twSpace, gain: 0.80, jitter: 0.03},
			Return: voice{clips: twBell, gain: 0.35, jitter: 0},
			Delete: voice{clips: twKey, gain: 0.70, jitter: 0.03},
		},
	}, nil
}

// basePitch shifts some kinds so they are distinguishable within a profile
// that has no dedicated sample for them (>1 is higher).
func basePitch(p Profile, k Kind) float64 {
	switch {
	case p == Typewriter:
		return 1
	case p == SoftClick || p == Clicky: // big keys are a little deeper
		switch k {
		case Space:
			return 0.86
		case Return:
			return 0.9
		case Delete:
			return 1.05
		}
		return 1
	case k == Space:
		return 0.85
	case k == Return:
		return 0.7
	case k == Delete:
		return 1.2
	}
	return 1
}

// output plays PCM: 32-bit float little-endian mono at SampleRate.
type output interface {
	play(pcm []byte)
}

// Player turns key presses into sounds. The zero value is not usable; call
// New. All methods are safe for concurrent use.
type Player struct {
	mu      sync.Mutex
	profile Profile
	volume  float64
	banks   map[Profile]bank
	rng     *rand.Rand
	last    map[Kind]int
	lastAt  time.Time
	now     func() time.Time

	open   func() (output, error) // opens the device; replaced in tests
	out    output
	opened bool

	// OnError is called once, from any goroutine, if the audio device
	// cannot be opened. Sounds are silently skipped after that.
	OnError func(error)
}

// New returns a silent player (profile Off) at volume 0.6.
func New() *Player {
	banks, err := loadBanks()
	if err != nil {
		// The assets are embedded, so this is a build defect.
		panic("sound: " + err.Error())
	}
	return &Player{
		profile: Off,
		volume:  0.6,
		banks:   banks,
		rng:     rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		last:    map[Kind]int{},
		now:     time.Now,
		open:    openDevice,
	}
}

// Profile returns the current profile.
func (p *Player) Profile() Profile {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.profile
}

// SetProfile switches the sound family; anything but Off opens the audio
// device on first use.
func (p *Player) SetProfile(pr Profile) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profile = ParseProfile(string(pr))
	if p.profile != Off {
		p.ensureOutput()
	}
}

// Volume returns the master volume in [0, 1].
func (p *Player) Volume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// SetVolume sets the master volume, clamped to [0, 1].
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = math.Max(0, math.Min(1, v))
}

// Play voices a key press with the current profile. It returns quickly and
// never blocks on the audio device.
func (p *Player) Play(k Kind) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.profile == Off || p.volume == 0 {
		return
	}
	now := p.now()
	if now.Sub(p.lastAt) < minInterval {
		return
	}
	p.lastAt = now
	p.ensureOutput()
	if p.out == nil {
		return
	}
	v, ok := p.banks[p.profile][k]
	if !ok {
		return
	}
	pcm := p.render(v, k)
	p.out.play(pcm)
}

// Preview plays a typical key sound regardless of the rate limit, for use
// right after the user picks a profile or volume.
func (p *Player) Preview() {
	p.mu.Lock()
	p.lastAt = time.Time{}
	p.mu.Unlock()
	p.Play(Key)
}

func (p *Player) ensureOutput() {
	if p.opened {
		return
	}
	p.opened = true
	out, err := p.open()
	if err != nil {
		if p.OnError != nil {
			go p.OnError(err)
		}
		return
	}
	p.out = out
}

// render picks a clip variant (never the same one twice in a row), resamples
// it for a small random pitch change and scales it to the target level.
func (p *Player) render(v voice, k Kind) []byte {
	i := 0
	if n := len(v.clips); n > 1 {
		i = p.rng.IntN(n - 1)
		if i >= p.last[k] {
			i++
		}
	}
	p.last[k] = i
	clip := v.clips[i]

	pitch := basePitch(p.profile, k) * (1 + v.jitter*(2*p.rng.Float64()-1))
	level := float32(p.volume) * v.gain * float32(0.85+0.15*p.rng.Float64())
	samples := resample(clip, pitch)

	pcm := make([]byte, 4*len(samples))
	for j, s := range samples {
		s *= level
		s = max(-1, min(1, s))
		binary.LittleEndian.PutUint32(pcm[4*j:], math.Float32bits(s))
	}
	return pcm
}

// resample plays src at the given speed ratio with linear interpolation;
// a ratio above 1 shortens the clip and raises its pitch.
func resample(src []float32, ratio float64) []float32 {
	if ratio <= 0 || len(src) == 0 {
		return nil
	}
	if ratio == 1 {
		out := make([]float32, len(src))
		copy(out, src)
		return out
	}
	n := int(float64(len(src)-1)/ratio) + 1
	out := make([]float32, n)
	for i := range out {
		pos := float64(i) * ratio
		j := int(pos)
		if j >= len(src)-1 {
			out[i] = src[len(src)-1]
			continue
		}
		f := float32(pos - float64(j))
		out[i] = src[j]*(1-f) + src[j+1]*f
	}
	return out
}
