package editor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Edit is the result of a formatting operation: the new text and the rune
// offset where the cursor should go.
type Edit struct {
	Text   string
	Cursor int
}

// Selection is a rune range [Start, End) in the text.
type Selection struct{ Start, End int }

func clamp(sel Selection, n int) Selection {
	if sel.Start > sel.End {
		sel.Start, sel.End = sel.End, sel.Start
	}
	sel.Start = max(0, min(sel.Start, n))
	sel.End = max(0, min(sel.End, n))
	return sel
}

// ToggleInline wraps the selection in marker (e.g. "**"), or removes the
// marker if the selection is already wrapped. With an empty selection the word
// under the cursor is wrapped; outside a word an empty pair is inserted with
// the cursor between the markers.
func ToggleInline(text string, sel Selection, marker string) Edit {
	r := []rune(text)
	sel = clamp(sel, len(r))
	m := []rune(marker)
	ml := len(m)

	if sel.Start == sel.End {
		ws, we := wordAt(r, sel.Start)
		if ws == we {
			out := splice(r, sel.Start, sel.End, append(append([]rune{}, m...), m...))
			return Edit{string(out), sel.Start + ml}
		}
		sel = Selection{ws, we}
	}

	// Keep surrounding whitespace outside the markers.
	for sel.Start < sel.End && unicode.IsSpace(r[sel.Start]) {
		sel.Start++
	}
	for sel.End > sel.Start && unicode.IsSpace(r[sel.End-1]) {
		sel.End--
	}
	inner := r[sel.Start:sel.End]

	// Selection includes the markers: "**bold**" -> "bold".
	if len(inner) >= 2*ml && hasMarker(inner, m, true) && hasMarker(inner, m, false) {
		body := inner[ml : len(inner)-ml]
		out := splice(r, sel.Start, sel.End, body)
		return Edit{string(out), sel.Start + len(body)}
	}
	// Markers just outside the selection: **[bold]** -> bold.
	if sel.Start >= ml && sel.End+ml <= len(r) &&
		hasMarker(r[sel.Start-ml:sel.Start], m, true) && hasMarker(r[sel.End:sel.End+ml], m, true) &&
		!isMarkerRun(r, sel.Start-ml-1, m) && !isMarkerRun(r, sel.End+ml, m) {
		out := splice(r, sel.End, sel.End+ml, nil)
		out = splice(out, sel.Start-ml, sel.Start, nil)
		return Edit{string(out), sel.End - ml}
	}

	wrapped := make([]rune, 0, len(inner)+2*ml)
	wrapped = append(wrapped, m...)
	wrapped = append(wrapped, inner...)
	wrapped = append(wrapped, m...)
	out := splice(r, sel.Start, sel.End, wrapped)
	return Edit{string(out), sel.Start + len(wrapped)}
}

// hasMarker reports whether s starts (prefix) or ends with m, treating a
// single "*" specially so that "**bold**" is not mistaken for italics.
func hasMarker(s, m []rune, prefix bool) bool {
	if len(s) < len(m) {
		return false
	}
	var part []rune
	if prefix {
		part = s[:len(m)]
	} else {
		part = s[len(s)-len(m):]
	}
	if string(part) != string(m) {
		return false
	}
	if string(m) == "*" {
		// Count the run of stars; italics are an odd count (1 or 3).
		n := 0
		if prefix {
			for n < len(s) && s[n] == '*' {
				n++
			}
		} else {
			for n < len(s) && s[len(s)-1-n] == '*' {
				n++
			}
		}
		return n%2 == 1
	}
	return true
}

func isMarkerRun(r []rune, i int, m []rune) bool {
	return i >= 0 && i < len(r) && len(m) == 1 && r[i] == m[0]
}

func wordAt(r []rune, pos int) (int, int) {
	isWord := func(c rune) bool { return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '\'' }
	s, e := pos, pos
	for s > 0 && isWord(r[s-1]) {
		s--
	}
	for e < len(r) && isWord(r[e]) {
		e++
	}
	return s, e
}

func splice(r []rune, start, end int, repl []rune) []rune {
	out := make([]rune, 0, len(r)-(end-start)+len(repl))
	out = append(out, r[:start]...)
	out = append(out, repl...)
	out = append(out, r[end:]...)
	return out
}

// lineRange expands sel to whole lines: it returns the rune offset of the
// first line's start and of the last line's end (excluding the newline).
func lineRange(r []rune, sel Selection) (int, int) {
	s, e := sel.Start, sel.End
	if e > s && e > 0 && r[e-1] == '\n' {
		e-- // selection ending at a line start does not include that line
	}
	for s > 0 && r[s-1] != '\n' {
		s--
	}
	for e < len(r) && r[e] != '\n' {
		e++
	}
	return s, e
}

// mapLines applies fn to every selected line and returns the edit with the
// cursor at the end of the last line.
func mapLines(text string, sel Selection, fn func(lines []string) []string) Edit {
	r := []rune(text)
	sel = clamp(sel, len(r))
	s, e := lineRange(r, sel)
	lines := strings.Split(string(r[s:e]), "\n")
	repl := []rune(strings.Join(fn(lines), "\n"))
	out := splice(r, s, e, repl)
	return Edit{string(out), s + len(repl)}
}

var headingPrefixRe = regexp.MustCompile(`^(#{1,6})[ \t]+`)

// SetHeading turns the selected lines into headings of the given level (1-6).
// Applying the same level again turns them back into plain text; level 0
// always removes the heading.
func SetHeading(text string, sel Selection, level int) Edit {
	return mapLines(text, sel, func(lines []string) []string {
		for i, l := range lines {
			cur := 0
			if m := headingPrefixRe.FindStringSubmatch(l); m != nil {
				cur = len(m[1])
				l = l[len(m[0]):]
			}
			if level > 0 && cur != level {
				l = strings.Repeat("#", level) + " " + l
			}
			lines[i] = l
		}
		return lines
	})
}

// LinePrefix is a kind of line-level formatting.
type LinePrefix int

// Line prefix kinds.
const (
	Bullet LinePrefix = iota
	Numbered
	Task
	Quote
)

var listPrefixRe = regexp.MustCompile(`^(\s*)(?:[-*]|\d+\.)[ \t]+(?:\[[ xX]\][ \t]+)?`)
var quotePrefixRe = regexp.MustCompile(`^>[ \t]?`)
var bulletRe = regexp.MustCompile(`^\s*[-*][ \t]+`)
var numberedRe = regexp.MustCompile(`^\s*\d+\.[ \t]+`)
var taskRe = regexp.MustCompile(`^\s*[-*][ \t]+\[[ xX]\][ \t]+`)

// ToggleLinePrefix applies (or removes, if every selected line already has it)
// a list or quote prefix on the selected lines. Converting between list kinds
// replaces the existing marker.
func ToggleLinePrefix(text string, sel Selection, kind LinePrefix) Edit {
	return mapLines(text, sel, func(lines []string) []string {
		has := func(l string) bool {
			switch kind {
			case Bullet:
				return bulletRe.MatchString(l) && !taskRe.MatchString(l)
			case Numbered:
				return numberedRe.MatchString(l)
			case Task:
				return taskRe.MatchString(l)
			default:
				return quotePrefixRe.MatchString(l)
			}
		}
		all := true
		for _, l := range lines {
			if strings.TrimSpace(l) != "" && !has(l) {
				all = false
			}
		}
		n := 0
		for i, l := range lines {
			blank := strings.TrimSpace(l) == ""
			if kind == Quote {
				switch {
				case all:
					lines[i] = quotePrefixRe.ReplaceAllString(l, "")
				case blank:
					lines[i] = ">"
				default:
					lines[i] = "> " + l
				}
				continue
			}
			if blank {
				continue
			}
			m := listPrefixRe.FindStringSubmatch(l)
			indent, body := "", l
			if m != nil {
				indent, body = m[1], l[len(m[0]):]
			} else {
				body = strings.TrimLeft(l, " ")
				indent = l[:len(l)-len(body)]
			}
			if all {
				lines[i] = indent + body
				continue
			}
			n++
			switch kind {
			case Bullet:
				lines[i] = indent + "- " + body
			case Numbered:
				lines[i] = fmt.Sprintf("%s%d. %s", indent, n, body)
			case Task:
				lines[i] = indent + "- [ ] " + body
			}
		}
		return lines
	})
}

// InsertBlock replaces the selection with a block element, making sure it is
// separated from surrounding text by blank lines. cursorAt is the rune offset
// inside block where the cursor should end up (-1 for the end of the block).
func InsertBlock(text string, sel Selection, block string, cursorAt int) Edit {
	r := []rune(text)
	sel = clamp(sel, len(r))
	before, after := r[:sel.Start], r[sel.End:]

	var pre string
	switch {
	case len(before) == 0:
	case strings.HasSuffix(string(before), "\n\n"):
	case before[len(before)-1] == '\n':
		pre = "\n"
	default:
		pre = "\n\n"
	}
	var post string
	switch {
	case len(after) == 0:
		post = "\n"
	case strings.HasPrefix(string(after), "\n\n"):
	case after[0] == '\n':
		post = "\n"
	default:
		post = "\n\n"
	}
	b := []rune(block)
	if cursorAt < 0 || cursorAt > len(b) {
		cursorAt = len(b)
	}
	ins := []rune(pre + block + post)
	out := splice(r, sel.Start, sel.End, ins)
	return Edit{string(out), sel.Start + len([]rune(pre)) + cursorAt}
}

// InsertText replaces the selection with s and puts the cursor after it, or
// at cursorAt runes into s when cursorAt >= 0.
func InsertText(text string, sel Selection, s string, cursorAt int) Edit {
	r := []rune(text)
	sel = clamp(sel, len(r))
	ins := []rune(s)
	if cursorAt < 0 || cursorAt > len(ins) {
		cursorAt = len(ins)
	}
	out := splice(r, sel.Start, sel.End, ins)
	return Edit{string(out), sel.Start + cursorAt}
}

// CodeBlock wraps the selection in a fenced code block (or inserts an empty
// one with the cursor inside).
func CodeBlock(text string, sel Selection, lang string) Edit {
	r := []rune(text)
	sel = clamp(sel, len(r))
	body := strings.Trim(string(r[sel.Start:sel.End]), "\n")
	open := "```" + lang + "\n"
	if body == "" {
		return InsertBlock(text, sel, open+"\n```", len([]rune(open)))
	}
	return InsertBlock(text, sel, open+body+"\n```", -1)
}

// Table builds a Markdown table skeleton with the given size.
func Table(cols, rows int, header bool) string {
	cols = max(cols, 1)
	rows = max(rows, 1)
	var b strings.Builder
	row := func(f func(c int) string) {
		b.WriteString("|")
		for c := 1; c <= cols; c++ {
			b.WriteString(" " + f(c) + " |")
		}
		b.WriteString("\n")
	}
	if header {
		row(func(c int) string { return fmt.Sprintf("Header %d", c) })
	}
	row(func(int) string { return "---" })
	for r := 1; r <= rows; r++ {
		row(func(c int) string { return fmt.Sprintf("Cell %d.%d", r, c) })
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// LinkMarkup builds [text](url "title").
func LinkMarkup(text, url, title string) string {
	if text == "" {
		text = url
	}
	s := "[" + escapeBrackets(text) + "](" + url
	if title != "" {
		s += ` "` + strings.ReplaceAll(title, `"`, "'") + `"`
	}
	return s + ")"
}

// ImageMarkup builds ![alt](src "title"), or !![caption](src) for media.
func ImageMarkup(alt, src, title string, media bool) string {
	prefix := "!"
	if media {
		prefix = "!!"
	}
	s := prefix + "[" + escapeBrackets(alt) + "](" + src
	if title != "" && !media {
		s += ` "` + strings.ReplaceAll(title, `"`, "'") + `"`
	}
	return s + ")"
}

func escapeBrackets(s string) string {
	return strings.NewReplacer("[", `\[`, "]", `\]`).Replace(s)
}

// Stats returns word and character counts.
func Stats(text string) (words, chars int) {
	return len(strings.Fields(text)), len([]rune(text))
}

var continueRe = regexp.MustCompile(`^(\s*)(?:([-*])|(\d+)\.)([ \t]+)(\[[ xX]\][ \t]+)?`)
var continueQuoteRe = regexp.MustCompile(`^>[ \t]?`)

// ContinueList handles Enter inside a list item or quote: it starts the next
// item with the same marker (numbers incremented, tasks unchecked). Enter on
// an empty item removes the marker and ends the list. It reports false when
// the cursor is not in a list or quote line.
func ContinueList(text string, cursor int) (Edit, bool) {
	r := []rune(text)
	if cursor < 0 || cursor > len(r) {
		return Edit{}, false
	}
	ls, le := lineRange(r, Selection{cursor, cursor})
	line := string(r[ls:le])

	var prefix, next string
	if m := continueRe.FindStringSubmatch(line); m != nil {
		prefix = m[0]
		marker := m[2]
		if m[3] != "" {
			n := 0
			fmt.Sscanf(m[3], "%d", &n)
			marker = fmt.Sprintf("%d.", n+1)
		}
		next = m[1] + marker + " "
		if m[5] != "" {
			next += "[ ] "
		}
	} else if m := continueQuoteRe.FindString(line); m != "" {
		prefix, next = m, "> "
	} else {
		return Edit{}, false
	}

	plen := len([]rune(prefix))
	if cursor-ls < plen {
		return Edit{}, false // cursor inside the marker itself
	}
	if strings.TrimSpace(line[len(prefix):]) == "" {
		// Empty item: end the list by clearing the marker.
		out := splice(r, ls, le, nil)
		return Edit{string(out), ls}, true
	}
	ins := []rune("\n" + next)
	out := splice(r, cursor, cursor, ins)
	return Edit{string(out), cursor + len(ins)}, true
}
