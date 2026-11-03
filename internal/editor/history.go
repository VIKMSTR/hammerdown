// Package editor holds GUI-independent editing logic: an undo/redo history and
// Markdown formatting operations on plain text.
package editor

import "time"

// Kind classifies an edit for undo grouping.
type Kind int

const (
	// Typing edits made within MergeWindow of each other are merged into one
	// undo step.
	Typing Kind = iota
	// Command edits (formatting, paste, ...) always form their own undo step.
	Command
)

// MergeWindow is the idle time after which typing starts a new undo step.
const MergeWindow = 800 * time.Millisecond

// maxGroup caps how long a single run of typing can be merged.
const maxGroup = 4 * time.Second

// History is a linear undo/redo stack of full text snapshots.
type History struct {
	states     []string
	idx        int
	lastKind   Kind
	lastEdit   time.Time
	groupStart time.Time
	limit      int
	now        func() time.Time
}

// NewHistory creates a history whose initial state is text.
func NewHistory(text string) *History {
	h := &History{limit: 1000, now: time.Now}
	h.Reset(text)
	return h
}

// Reset clears the history and makes text the only state.
func (h *History) Reset(text string) {
	h.states = []string{text}
	h.idx = 0
	h.lastKind = Command
}

// Current returns the text at the current position in the history.
func (h *History) Current() string { return h.states[h.idx] }

// Record registers that the text changed to text. It is a no-op if the text is
// unchanged.
func (h *History) Record(text string, kind Kind) {
	if text == h.states[h.idx] {
		return
	}
	now := h.now()
	h.states = h.states[:h.idx+1]
	merge := kind == Typing && h.lastKind == Typing && h.idx > 0 &&
		now.Sub(h.lastEdit) < MergeWindow && now.Sub(h.groupStart) < maxGroup
	if merge {
		h.states[h.idx] = text
	} else {
		h.states = append(h.states, text)
		h.idx++
		h.groupStart = now
		if len(h.states) > h.limit {
			drop := len(h.states) - h.limit
			h.states = append([]string(nil), h.states[drop:]...)
			h.idx -= drop
		}
	}
	h.lastKind = kind
	h.lastEdit = now
}

// Break ends the current typing group so the next edit starts a new step.
func (h *History) Break() { h.lastKind = Command }

// CanUndo reports whether Undo would do anything.
func (h *History) CanUndo() bool { return h.idx > 0 }

// CanRedo reports whether Redo would do anything.
func (h *History) CanRedo() bool { return h.idx < len(h.states)-1 }

// Undo steps back and returns the previous text.
func (h *History) Undo() (string, bool) {
	if !h.CanUndo() {
		return h.states[h.idx], false
	}
	h.idx--
	h.lastKind = Command
	return h.states[h.idx], true
}

// Redo steps forward and returns the next text.
func (h *History) Redo() (string, bool) {
	if !h.CanRedo() {
		return h.states[h.idx], false
	}
	h.idx++
	h.lastKind = Command
	return h.states[h.idx], true
}

// ChangeEnd returns the rune offset in to just after the region where from
// and to differ — a natural cursor position after undo/redo.
func ChangeEnd(from, to string) int {
	a, b := []rune(from), []rune(to)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	return len(b) - s
}
