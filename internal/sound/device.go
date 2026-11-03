package sound

import (
	"bytes"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// device is the real output: one oto context mixing short-lived players.
type device struct {
	ctx *oto.Context

	mu     sync.Mutex
	voices []*oto.Player // kept reachable while playing (oto stops collected players)
}

// openDevice opens the system audio output. Initialisation finishes in the
// background; sounds requested before it is ready are dropped.
func openDevice() (output, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:      SampleRate,
		ChannelCount:    1,
		Format:          oto.FormatFloat32LE,
		BufferSize:      20 * time.Millisecond, // keep key-to-sound latency low
		ApplicationName: "hammerdown",
	})
	if err != nil {
		return nil, err
	}
	d := &device{}
	go func() {
		<-ready
		if ctx.Err() != nil {
			return
		}
		d.mu.Lock()
		d.ctx = ctx
		d.mu.Unlock()
	}()
	return d, nil
}

func (d *device) play(pcm []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx == nil || d.ctx.Err() != nil {
		return
	}
	live := d.voices[:0]
	for _, v := range d.voices {
		if v.IsPlaying() {
			live = append(live, v)
		}
	}
	clear(d.voices[len(live):])
	d.voices = live
	if len(d.voices) >= maxVoices {
		d.voices[0].Pause()
		d.voices = d.voices[1:]
	}
	pl := d.ctx.NewPlayer(bytes.NewReader(pcm))
	pl.Play()
	d.voices = append(d.voices, pl)
}
