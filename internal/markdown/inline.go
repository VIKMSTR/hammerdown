package markdown

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// escapable lists the characters that a backslash can escape.
const escapable = "*`~[]!\\"

// maxInlineDepth bounds nesting of emphasis/links so pathological input
// cannot blow the stack.
const maxInlineDepth = 32

// ParseInlines parses a single logical line of inline Markdown. A "\n" in the
// input is a hard line break.
func ParseInlines(s string) []Inline {
	p := &inlineParser{s: s, failed: map[failKey]bool{}}
	nodes, _, _ := p.parse(0, "", 0)
	return mergeText(nodes)
}

type failKey struct {
	pos    int
	closer string
}

type inlineParser struct {
	s string
	// failed memoizes (position, closer) pairs that never found their closer,
	// keeping the backtracking parser roughly O(n^2) instead of exponential.
	failed map[failKey]bool
}

// parse consumes inline content starting at i until it finds closer (if
// non-empty). It returns the nodes, the index after the closer and whether the
// closer was found.
func (p *inlineParser) parse(i int, closer string, depth int) ([]Inline, int, bool) {
	s := p.s
	start := i
	var nodes []Inline
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			nodes = append(nodes, &Text{Text: buf.String()})
			buf.Reset()
		}
	}

	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\':
			if i+1 < len(s) && strings.IndexByte(escapable, s[i+1]) >= 0 {
				buf.WriteByte(s[i+1])
				i += 2
				continue
			}
			buf.WriteByte('\\')
			i++

		case c == '\n':
			flush()
			nodes = append(nodes, &LineBreak{})
			i++

		case c == '`':
			n := runLen(s, i, '`')
			fence := s[i : i+n]
			if j := strings.Index(s[i+n:], fence); j >= 0 {
				flush()
				code := s[i+n : i+n+j]
				if n > 1 {
					code = strings.TrimSpace(code)
				}
				nodes = append(nodes, &Code{Text: code})
				i += n + j + n
				continue
			}
			buf.WriteString(fence)
			i += n

		case c == '~' && strings.HasPrefix(s[i:], "~~"):
			if closer == "~~" && i > start && canClose(s, i) {
				flush()
				return nodes, i + 2, true
			}
			if depth < maxInlineDepth && canOpen(s, i+2) {
				if kids, end, ok := p.try(i+2, "~~", depth); ok {
					flush()
					nodes = append(nodes, &Strike{Children: kids})
					i = end
					continue
				}
			}
			buf.WriteString("~~")
			i += 2

		case c == '*':
			n := runLen(s, i, '*')
			if (closer == "*" || closer == "**") && i > start && canClose(s, i) && n >= len(closer) {
				flush()
				return nodes, i + len(closer), true
			}
			if depth < maxInlineDepth && canOpen(s, i+n) {
				if n >= 2 {
					if kids, end, ok := p.try(i+2, "**", depth); ok {
						flush()
						nodes = append(nodes, &Strong{Children: kids})
						i = end
						continue
					}
				}
				if kids, end, ok := p.try(i+1, "*", depth); ok {
					flush()
					nodes = append(nodes, &Emphasis{Children: kids})
					i = end
					continue
				}
				buf.WriteByte('*')
				i++
				continue
			}
			buf.WriteString(s[i : i+n])
			i += n

		case c == '[':
			if link, end, ok := p.link(i, depth); ok {
				flush()
				nodes = append(nodes, link)
				i = end
				continue
			}
			buf.WriteByte('[')
			i++

		case c == 'h' && (strings.HasPrefix(s[i:], "https://") || strings.HasPrefix(s[i:], "http://")) && wordBoundaryBefore(s, i):
			url := scanURL(s[i:])
			flush()
			nodes = append(nodes, &Link{Href: url, Children: []Inline{&Text{Text: url}}})
			i += len(url)

		default:
			buf.WriteByte(c)
			i++
		}
	}
	flush()
	return nodes, i, closer == ""
}

// try runs a nested parse for closer, remembering failures.
func (p *inlineParser) try(i int, closer string, depth int) ([]Inline, int, bool) {
	key := failKey{i, closer}
	if p.failed[key] {
		return nil, 0, false
	}
	kids, end, ok := p.parse(i, closer, depth+1)
	if !ok || len(kids) == 0 {
		p.failed[key] = true
		return nil, 0, false
	}
	return mergeText(kids), end, true
}

// link parses [text](href "title") starting at the '['.
func (p *inlineParser) link(i, depth int) (Inline, int, bool) {
	s := p.s
	labelEnd := matchBracket(s, i)
	if labelEnd < 0 || labelEnd+1 >= len(s) || s[labelEnd+1] != '(' {
		return nil, 0, false
	}
	href, title, end, ok := parseDestination(s, labelEnd+1)
	if !ok {
		return nil, 0, false
	}
	label := s[i+1 : labelEnd]
	var kids []Inline
	if depth < maxInlineDepth {
		sub := &inlineParser{s: label, failed: map[failKey]bool{}}
		kids, _, _ = sub.parse(0, "", depth+1)
		kids = mergeText(kids)
	} else {
		kids = []Inline{&Text{Text: label}}
	}
	if len(kids) == 0 {
		kids = []Inline{&Text{Text: href}}
	}
	return &Link{Href: href, Title: title, Children: kids}, end, true
}

// matchBracket returns the index of the ']' matching the '[' at i, honouring
// escapes, nesting and code spans, or -1.
func matchBracket(s string, i int) int {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '`':
			n := runLen(s, j, '`')
			if k := strings.Index(s[j+n:], s[j:j+n]); k >= 0 {
				j += n + k + n - 1
			} else {
				j += n - 1
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j
			}
		case '\n':
			return -1
		}
	}
	return -1
}

// parseDestination parses `(href "optional title")` starting at the '('.
func parseDestination(s string, i int) (href, title string, end int, ok bool) {
	j := i + 1
	for j < len(s) && s[j] == ' ' {
		j++
	}
	k := j
	parens := 0
	for k < len(s) {
		c := s[k]
		if c == ' ' || c == '\n' {
			break
		}
		if c == '(' {
			parens++
		} else if c == ')' {
			if parens == 0 {
				break
			}
			parens--
		}
		k++
	}
	href = s[j:k]
	for k < len(s) && s[k] == ' ' {
		k++
	}
	if k < len(s) && (s[k] == '"' || s[k] == '\'') {
		q := s[k]
		e := strings.IndexByte(s[k+1:], q)
		if e < 0 {
			return "", "", 0, false
		}
		title = s[k+1 : k+1+e]
		k += e + 2
		for k < len(s) && s[k] == ' ' {
			k++
		}
	}
	if k >= len(s) || s[k] != ')' || href == "" {
		return "", "", 0, false
	}
	return href, title, k + 1, true
}

// scanURL returns the bare URL at the start of s, without trailing punctuation.
func scanURL(s string) string {
	end := strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '<' || r == '>' })
	if end < 0 {
		end = len(s)
	}
	u := s[:end]
	for len(u) > 0 {
		last := u[len(u)-1]
		if strings.IndexByte(".,;:!?'\"*~", last) >= 0 {
			u = u[:len(u)-1]
			continue
		}
		if last == ')' && strings.Count(u, "(") < strings.Count(u, ")") {
			u = u[:len(u)-1]
			continue
		}
		break
	}
	return u
}

func runLen(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// canOpen reports whether the character at i (just after an opening
// delimiter) is non-whitespace.
func canOpen(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return !unicode.IsSpace(r)
}

// canClose reports whether the character before i (just before a closing
// delimiter) is non-whitespace.
func canClose(s string, i int) bool {
	if i == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !unicode.IsSpace(r)
}

func wordBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '/'
}

func mergeText(in []Inline) []Inline {
	out := in[:0:0]
	for _, n := range in {
		if t, ok := n.(*Text); ok && len(out) > 0 {
			if prev, ok := out[len(out)-1].(*Text); ok {
				out[len(out)-1] = &Text{Text: prev.Text + t.Text}
				continue
			}
		}
		out = append(out, n)
	}
	return out
}
