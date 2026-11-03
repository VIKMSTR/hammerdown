package ui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"hammerdown/internal/editor"
)

const (
	meterSegments = 20
	meterMax      = 800.0 // TPM at full scale; red only for real bursts
	meterFrame    = time.Second / 30
	meterHold     = 1200 * time.Millisecond // peak marker hold before it falls
	meterFall     = 300.0                   // peak fall speed, TPM per second
	meterAttack   = 0.35                    // easing towards a higher reading
	meterRelease  = 0.10                    // easing towards a lower reading
)

// meterStops map a position on the scale (0–1) to a colour: solid green for
// the first half, amber around 80 %, red only at the very top, like an audio
// level meter.
var meterStops = []struct {
	at float64
	c  color.NRGBA
}{
	{0, rgb(0x2fbf5b)},
	{0.5, rgb(0x2fbf5b)},
	{0.8, rgb(0xf2c230)},
	{1, rgb(0xe8433a)},
}

// segmentColor is the lit colour of segment i of n.
func segmentColor(i, n int) color.NRGBA {
	t := float64(i) / float64(n-1)
	j := 1
	for j < len(meterStops)-1 && t > meterStops[j].at {
		j++
	}
	a, b := meterStops[j-1], meterStops[j]
	f := (t - a.at) / (b.at - a.at)
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*f + 0.5) }
	return color.NRGBA{R: mix(a.c.R, b.c.R), G: mix(a.c.G, b.c.G), B: mix(a.c.B, b.c.B), A: 0xff}
}

// speedMeter shows typing speed in keystrokes ("types") per minute as an
// animated segmented bar with a peak-hold marker and a number. It animates
// only while there is something to show, so it costs nothing when idle.
type speedMeter struct {
	widget.BaseWidget

	speed   editor.Speed
	display float64 // eased reading shown by the bar
	peak    float64
	peakAt  time.Time
	shown   int // last number drawn, to skip redundant refreshes
	now     func() time.Time
	onHover func(bool)

	running bool
	stop    chan struct{}
}

func newSpeedMeter(hover func(bool)) *speedMeter {
	m := &speedMeter{now: time.Now, onHover: hover, shown: -1}
	m.ExtendBaseWidget(m)
	return m
}

// record counts a keystroke and wakes the animation.
func (m *speedMeter) record() {
	m.speed.Record(m.now())
	m.start()
}

// reset clears the reading and stops the animation (meter hidden).
func (m *speedMeter) reset() {
	m.halt()
	m.speed.Reset()
	m.display, m.peak, m.shown = 0, 0, -1
	m.Refresh()
}

func (m *speedMeter) start() {
	if m.running {
		return
	}
	m.running = true
	m.stop = make(chan struct{})
	go func(stop chan struct{}) {
		tk := time.NewTicker(meterFrame)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				fyne.Do(func() {
					select {
					case <-stop: // halted while this frame was queued
					default:
						m.tick()
					}
				})
			}
		}
	}(m.stop)
}

func (m *speedMeter) halt() {
	if m.running {
		close(m.stop)
		m.running = false
	}
}

// tick advances the animation by one frame; it runs on the UI goroutine.
func (m *speedMeter) tick() {
	now := m.now()
	target := m.speed.PerMinute(now)
	k := meterRelease
	if target > m.display {
		k = meterAttack
	}
	m.display += (target - m.display) * k
	if math.Abs(target-m.display) < 0.5 {
		m.display = target
	}

	if m.display >= m.peak {
		m.peak, m.peakAt = m.display, now
	} else if now.Sub(m.peakAt) > meterHold {
		m.peak = max(m.display, m.peak-meterFall*meterFrame.Seconds())
	}

	m.Refresh()
	if target == 0 && m.display == 0 && m.peak == 0 {
		m.halt() // idle: sleep until the next keystroke
	}
}

func (m *speedMeter) MouseIn(*desktop.MouseEvent) {
	if m.onHover != nil {
		m.onHover(true)
	}
}

func (m *speedMeter) MouseMoved(*desktop.MouseEvent) {}

func (m *speedMeter) MouseOut() {
	if m.onHover != nil {
		m.onHover(false)
	}
}

func (m *speedMeter) CreateRenderer() fyne.WidgetRenderer {
	r := &speedMeterRenderer{m: m}
	for range meterSegments {
		s := canvas.NewRectangle(color.Transparent)
		s.CornerRadius = 1.5
		r.segs = append(r.segs, s)
	}
	r.text = canvas.NewText("", theme.Color(theme.ColorNameForeground))
	r.text.TextStyle = fyne.TextStyle{Monospace: true}
	r.unit = canvas.NewText("TPM", themeColor(colorMuted))
	r.Refresh()
	return r
}

type speedMeterRenderer struct {
	m          *speedMeter
	segs       []*canvas.Rectangle
	text, unit *canvas.Text
}

const (
	segW, segH, segGap = float32(4), float32(11), float32(2)
)

func (r *speedMeterRenderer) barWidth() float32 {
	return meterSegments*segW + (meterSegments-1)*segGap
}

func (r *speedMeterRenderer) Layout(s fyne.Size) {
	p := theme.Padding()
	y := (s.Height - segH) / 2
	for i, seg := range r.segs {
		seg.Move(fyne.NewPos(p+float32(i)*(segW+segGap), y))
		seg.Resize(fyne.NewSize(segW, segH))
	}
	x := p + r.barWidth() + p*1.5
	ts, us := r.text.MinSize(), r.unit.MinSize()
	r.text.Move(fyne.NewPos(x, (s.Height-ts.Height)/2))
	r.unit.Move(fyne.NewPos(x+ts.Width+p/2, (s.Height-us.Height)/2))
}

func (r *speedMeterRenderer) MinSize() fyne.Size {
	p := theme.Padding()
	ts, us := r.text.MinSize(), r.unit.MinSize()
	return fyne.NewSize(p+r.barWidth()+p*1.5+ts.Width+p/2+us.Width+p,
		max(segH, ts.Height, us.Height)+p)
}

func (r *speedMeterRenderer) Refresh() {
	m := r.m
	size := theme.CaptionTextSize()
	r.text.TextSize, r.unit.TextSize = size, size
	r.text.Color = theme.Color(theme.ColorNameForeground)
	r.unit.Color = themeColor(colorMuted)
	n := int(math.Round(m.display))
	if n != m.shown {
		r.text.Text = fmt.Sprintf("%3d", n)
		m.shown = n
	}

	level := min(m.display/meterMax, 1) * meterSegments
	peakSeg := -1
	if m.peak > 0 {
		peakSeg = min(int(m.peak/meterMax*meterSegments), meterSegments-1)
	}
	for i, seg := range r.segs {
		c := segmentColor(i, meterSegments)
		lit := math.Max(0, math.Min(1, level-float64(i))) // fractional top segment
		if i == peakSeg {
			lit = 1
		}
		const dim = 0.16
		c.A = uint8(255 * (dim + (1-dim)*lit))
		seg.FillColor = c
		seg.Refresh()
	}
	r.text.Refresh()
	r.unit.Refresh()
}

func (r *speedMeterRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(r.segs)+2)
	for _, s := range r.segs {
		objs = append(objs, s)
	}
	return append(objs, r.text, r.unit)
}

func (r *speedMeterRenderer) Destroy() {}
