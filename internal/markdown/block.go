package markdown

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	headingRe    = regexp.MustCompile(`^(#{1,6})[ \t]+(.*?)[ \t]*$`)
	listRe       = regexp.MustCompile(`^( *)([-*]|\d+\.)[ \t]+(.*)$`)
	imageRe      = regexp.MustCompile(`^!\[((?:\\.|[^\]])*)\]\((\S+?)(?:[ \t]+"([^"]*)")?\)$`)
	mediaRe      = regexp.MustCompile(`^!!\[((?:\\.|[^\]])*)\]\((\S+?)(?:[ \t]+"([^"]*)")?\)$`)
	attachmentRe = regexp.MustCompile(`^\[((?:\\.|[^\]])*)\]\(([^\s()]+)\)$`)
	metaRe       = regexp.MustCompile(`^([A-Za-z_][\w-]*):[ \t]*(.*)$`)
	sepCellRe    = regexp.MustCompile(`^:?-+:?$`)
)

// TeaserMarker is the line that splits teaser from the rest of a post.
const TeaserMarker = "//--more--//"

var attachmentExt = map[string]bool{
	".pdf": true, ".zip": true, ".tgz": true, ".epub": true, ".txt": true,
	".md": true, ".ics": true, ".gpx": true, ".csv": true,
}

var audioExt = map[string]bool{
	".mp3": true, ".m4a": true, ".ogg": true, ".opus": true, ".aac": true,
	".flac": true, ".wav": true,
}

// Parse parses a complete Markdown document.
func Parse(src string) *Document {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	src = strings.ReplaceAll(src, "\t", "    ")
	lines := strings.Split(src, "\n")

	doc := &Document{}
	lines = parseFrontMatter(lines, doc)
	doc.Blocks = parseBlocks(lines)
	assignIDs(doc.Blocks, map[string]int{})
	return doc
}

// parseFrontMatter extracts a leading "---" ... "---" block made only of
// "key: value" lines and returns the remaining lines.
func parseFrontMatter(lines []string, doc *Document) []string {
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return lines
	}
	var meta []MetaField
	for i := 1; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if l == "---" {
			if len(meta) == 0 {
				return lines
			}
			doc.Meta = meta
			return lines[i+1:]
		}
		if l == "" {
			continue
		}
		m := metaRe.FindStringSubmatch(l)
		if m == nil {
			return lines
		}
		meta = append(meta, MetaField{Key: m[1], Value: strings.TrimSpace(m[2])})
	}
	return lines
}

func parseBlocks(lines []string) []Block {
	var blocks []Block
	i := 0
	for i < len(lines) {
		line := lines[i]
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			i++
		case strings.HasPrefix(t, "```"):
			var b Block
			b, i = parseFence(lines, i)
			blocks = append(blocks, b)
		case strings.HasPrefix(t, ":::"):
			name := strings.TrimSpace(t[3:])
			if name == "end" || name == "" {
				i++ // stray terminator
				continue
			}
			var b Block
			b, i = parseContainer(lines, i, name)
			blocks = append(blocks, b)
		case t == TeaserMarker:
			blocks = append(blocks, &Teaser{})
			i++
		case isHR(t):
			blocks = append(blocks, &HorizontalRule{})
			i++
		case headingRe.MatchString(t) && !startsIndented(line):
			m := headingRe.FindStringSubmatch(t)
			blocks = append(blocks, &Heading{Level: len(m[1]), Inlines: ParseInlines(m[2])})
			i++
		case strings.HasPrefix(t, ">"):
			var b Block
			b, i = parseQuote(lines, i)
			blocks = append(blocks, b)
		case isTableStart(lines, i):
			var b Block
			b, i = parseTable(lines, i)
			blocks = append(blocks, b)
		case mediaRe.MatchString(t):
			blocks = append(blocks, parseMedia(t))
			i++
		case imageRe.MatchString(t):
			m := imageRe.FindStringSubmatch(t)
			blocks = append(blocks, &Image{Alt: unescape(m[1]), Src: m[2], Title: m[3]})
			i++
		case isAttachment(t):
			m := attachmentRe.FindStringSubmatch(t)
			blocks = append(blocks, &Attachment{Text: unescape(m[1]), Href: m[2]})
			i++
		case listRe.MatchString(line):
			var bs []Block
			bs, i = parseList(lines, i)
			blocks = append(blocks, bs...)
		default:
			var b Block
			b, i = parseParagraph(lines, i)
			blocks = append(blocks, b)
		}
	}
	return groupGalleries(blocks)
}

func startsIndented(line string) bool {
	return strings.HasPrefix(line, "    ")
}

func isHR(t string) bool {
	return len(t) >= 3 && strings.Trim(t, "-") == ""
}

// startsBlock reports whether line would begin a block other than a paragraph,
// and therefore interrupts a running paragraph.
func startsBlock(lines []string, i int) bool {
	line := lines[i]
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, ":::") ||
		t == TeaserMarker || isHR(t) || headingRe.MatchString(t) ||
		strings.HasPrefix(t, ">") || isTableStart(lines, i) ||
		mediaRe.MatchString(t) || imageRe.MatchString(t) || listRe.MatchString(line)
}

func parseFence(lines []string, i int) (Block, int) {
	open := lines[i]
	indent := len(open) - len(strings.TrimLeft(open, " "))
	lang := strings.TrimSpace(strings.TrimSpace(open)[3:])
	var body []string
	j := i + 1
	for ; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "```" {
			j++
			break
		}
		body = append(body, trimIndent(lines[j], indent))
	}
	if lang == "chat" {
		return parseChat(body), j
	}
	return &CodeBlock{Lang: lang, Text: strings.Join(body, "\n")}, j
}

func trimIndent(s string, n int) string {
	for n > 0 && strings.HasPrefix(s, " ") {
		s = s[1:]
		n--
	}
	return s
}

func parseChat(body []string) *Chat {
	c := &Chat{}
	var texts []string
	commit := func() {
		if len(c.Lines) > 0 {
			c.Lines[len(c.Lines)-1].Inlines = ParseInlines(strings.Join(texts, " "))
		}
	}
	for _, l := range body {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if speaker, text, ok := splitSpeaker(l); ok {
			commit()
			c.Lines = append(c.Lines, ChatLine{Speaker: speaker})
			texts = []string{text}
			continue
		}
		if len(c.Lines) == 0 {
			c.Lines = append(c.Lines, ChatLine{})
		}
		texts = append(texts, l)
	}
	commit()
	return c
}

// splitSpeaker splits "Name: statement".
func splitSpeaker(l string) (string, string, bool) {
	idx := strings.IndexByte(l, ':')
	if idx <= 0 || idx > 40 {
		return "", "", false
	}
	name := strings.TrimSpace(l[:idx])
	rest := l[idx+1:]
	if name == "" || strings.ContainsAny(name, "/[]()*`") || (rest != "" && rest[0] != ' ') {
		return "", "", false
	}
	return name, strings.TrimSpace(rest), true
}

func parseContainer(lines []string, i int, name string) (Block, int) {
	depth := 1
	j := i + 1
	inFence := false
	for ; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
		}
		if inFence || !strings.HasPrefix(t, ":::") {
			continue
		}
		n := strings.TrimSpace(t[3:])
		if n == "end" {
			depth--
			if depth == 0 {
				break
			}
		} else if n != "" {
			depth++
		}
	}
	end := j
	if end > len(lines) {
		end = len(lines)
	}
	c := &Container{Name: name, Blocks: parseBlocks(lines[i+1 : end])}
	return c, j + 1
}

func parseQuote(lines []string, i int) (Block, int) {
	var content []string
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, ">") {
			break
		}
		t = strings.TrimPrefix(t[1:], " ")
		content = append(content, t)
	}
	q := &Blockquote{}
	if n := len(content); n > 1 {
		if a := attribution(content[n-1]); a != "" {
			q.Attribution = ParseInlines(a)
			content = content[:n-1]
		}
	}
	var para []string
	flush := func() {
		if len(para) > 0 {
			q.Paragraphs = append(q.Paragraphs, ParseInlines(joinLines(para)))
			para = nil
		}
	}
	for _, l := range content {
		if strings.TrimSpace(l) == "" {
			flush()
			continue
		}
		para = append(para, l)
	}
	flush()
	return q, i
}

// attribution returns the text after a leading em dash or "--", or "".
func attribution(l string) string {
	for _, p := range []string{"—", "--"} {
		if strings.HasPrefix(l, p) {
			if rest := strings.TrimSpace(strings.TrimPrefix(l, p)); rest != "" {
				return rest
			}
		}
	}
	return ""
}

func isTableStart(lines []string, i int) bool {
	t := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(t, "|") {
		return false
	}
	if isSeparatorRow(t) && strings.HasSuffix(t, "|") && len(t) > 1 {
		return true // header-less table
	}
	return i+1 < len(lines) && isSeparatorRow(strings.TrimSpace(lines[i+1]))
}

func isSeparatorRow(t string) bool {
	if !strings.Contains(t, "-") || !strings.Contains(t, "|") {
		return false
	}
	for _, c := range splitRow(t) {
		if !sepCellRe.MatchString(c) {
			return false
		}
	}
	return true
}

// splitRow splits a "| a | b |" row into trimmed cells, ignoring pipes that are
// escaped or inside code spans.
func splitRow(t string) []string {
	t = strings.TrimSpace(t)
	t = strings.TrimPrefix(t, "|")
	if strings.HasSuffix(t, "|") && !strings.HasSuffix(t, "\\|") {
		t = t[:len(t)-1]
	}
	var cells []string
	var cur strings.Builder
	inCode := false
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case c == '\\' && i+1 < len(t) && t[i+1] == '|':
			cur.WriteByte('|')
			i++
		case c == '`':
			inCode = !inCode
			cur.WriteByte(c)
		case c == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	return cells
}

func parseTable(lines []string, i int) (Block, int) {
	tbl := &Table{}
	first := strings.TrimSpace(lines[i])
	var sep string
	if isSeparatorRow(first) {
		sep = first
		i++
	} else {
		for _, c := range splitRow(first) {
			tbl.Header = append(tbl.Header, ParseInlines(c))
		}
		sep = strings.TrimSpace(lines[i+1])
		i += 2
	}
	for _, c := range splitRow(sep) {
		left, right := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":")
		switch {
		case left && right:
			tbl.Align = append(tbl.Align, AlignCenter)
		case right:
			tbl.Align = append(tbl.Align, AlignRight)
		case left:
			tbl.Align = append(tbl.Align, AlignLeft)
		default:
			tbl.Align = append(tbl.Align, AlignDefault)
		}
	}
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, "|") {
			break
		}
		var row [][]Inline
		for _, c := range splitRow(t) {
			row = append(row, ParseInlines(c))
		}
		tbl.Rows = append(tbl.Rows, row)
	}
	return tbl, i
}

func parseMedia(t string) *Media {
	m := mediaRe.FindStringSubmatch(t)
	media := &Media{Caption: unescape(m[1]), Src: m[2]}
	media.Kind, media.Provider = classifyMedia(media.Src)
	return media
}

func classifyMedia(src string) (MediaKind, string) {
	u, err := url.Parse(src)
	if err == nil && u.Host != "" {
		host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
		switch {
		case host == "youtube.com" || host == "m.youtube.com" || host == "youtu.be":
			return MediaVideo, "youtube"
		case strings.HasSuffix(host, "vimeo.com"):
			return MediaVideo, "vimeo"
		case host == "archive.org":
			return MediaVideo, "archive.org"
		case strings.HasSuffix(host, "spotify.com"):
			return MediaAudio, "spotify"
		case strings.HasSuffix(host, "soundcloud.com"):
			return MediaAudio, "soundcloud"
		case strings.HasSuffix(host, "mixcloud.com"):
			return MediaAudio, "mixcloud"
		case strings.HasSuffix(host, "bandcamp.com"):
			return MediaAudio, "bandcamp"
		case strings.Contains(host, "funkwhale"):
			return MediaAudio, "funkwhale"
		case strings.HasPrefix(u.Path, "/w/") || strings.HasPrefix(u.Path, "/videos/watch/"):
			return MediaVideo, "peertube"
		}
		if audioExt[strings.ToLower(path.Ext(u.Path))] {
			return MediaAudio, ""
		}
		return MediaVideo, ""
	}
	if audioExt[strings.ToLower(path.Ext(src))] {
		return MediaAudio, ""
	}
	return MediaVideo, ""
}

// YouTubeID extracts the video ID from a YouTube address, or "".
func YouTubeID(src string) string {
	u, err := url.Parse(src)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	switch host {
	case "youtu.be":
		return strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com":
		if v := u.Query().Get("v"); v != "" {
			return v
		}
		for _, p := range []string{"/embed/", "/shorts/", "/live/"} {
			if strings.HasPrefix(u.Path, p) {
				return strings.Trim(strings.TrimPrefix(u.Path, p), "/")
			}
		}
	}
	return ""
}

func isAttachment(t string) bool {
	m := attachmentRe.FindStringSubmatch(t)
	if m == nil {
		return false
	}
	href := m[2]
	if strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "#") {
		return false
	}
	return attachmentExt[strings.ToLower(path.Ext(href))]
}

type listLine struct {
	indent  int
	ordered bool
	text    string
}

func parseList(lines []string, i int) ([]Block, int) {
	var items []listLine
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			break
		}
		if m := listRe.FindStringSubmatch(line); m != nil && !isHR(strings.TrimSpace(line)) {
			items = append(items, listLine{
				indent:  len(m[1]),
				ordered: m[2] != "-" && m[2] != "*",
				text:    m[3],
			})
			continue
		}
		// Continuation of a wrapped bullet: indented by at least two spaces.
		if strings.HasPrefix(line, "  ") && len(items) > 0 && !startsBlock(lines, i) {
			last := &items[len(items)-1]
			last.text = joinLines([]string{last.text, line})
			continue
		}
		break
	}
	// A change of marker type at the top level starts a new sibling list.
	var lists []Block
	for pos := 0; pos < len(items); {
		lists = append(lists, buildList(items, &pos, items[pos].indent))
	}
	return lists, i
}

// buildList consumes items at the given indent (and their deeper children).
func buildList(items []listLine, pos *int, indent int) *List {
	l := &List{Ordered: items[*pos].ordered}
	for *pos < len(items) {
		it := items[*pos]
		if it.indent < indent {
			break
		}
		if it.indent > indent {
			// Deeper than expected: nest under the previous item (or this list).
			child := buildList(items, pos, it.indent)
			if len(l.Items) == 0 {
				l.Items = append(l.Items, &ListItem{})
			}
			last := l.Items[len(l.Items)-1]
			last.Children = append(last.Children, child)
			continue
		}
		if it.ordered != l.Ordered && len(l.Items) > 0 {
			// A different marker at the same level ends this list; the caller
			// starts a sibling list.
			break
		}
		li := &ListItem{}
		text := it.text
		switch {
		case strings.HasPrefix(text, "[ ] ") || text == "[ ]":
			li.Task = true
			text = strings.TrimPrefix(strings.TrimPrefix(text, "[ ]"), " ")
		case strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ") || text == "[x]" || text == "[X]":
			li.Task, li.Checked = true, true
			text = strings.TrimPrefix(text[3:], " ")
		}
		li.Inlines = ParseInlines(text)
		l.Items = append(l.Items, li)
		*pos++
	}
	return l
}

func parseParagraph(lines []string, i int) (Block, int) {
	var para []string
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			break
		}
		if len(para) > 0 && startsBlock(lines, i) {
			break
		}
		para = append(para, lines[i])
	}
	return &Paragraph{Inlines: ParseInlines(joinLines(para))}, i
}

// joinLines joins source lines into one inline string. A line ending in an
// (unescaped) backslash produces a hard break ("\n"); others join with a space.
func joinLines(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		l = strings.TrimSpace(l)
		hard := false
		if n := len(l) - len(strings.TrimRight(l, "\\")); n%2 == 1 {
			l = strings.TrimRight(l[:len(l)-1], " ")
			hard = true
		}
		b.WriteString(l)
		if i < len(lines)-1 {
			if hard {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
		}
	}
	return b.String()
}

func groupGalleries(blocks []Block) []Block {
	var out []Block
	for i := 0; i < len(blocks); i++ {
		img, ok := blocks[i].(*Image)
		if !ok {
			out = append(out, blocks[i])
			continue
		}
		g := &Gallery{Images: []*Image{img}}
		for i+1 < len(blocks) {
			next, ok := blocks[i+1].(*Image)
			if !ok {
				break
			}
			g.Images = append(g.Images, next)
			i++
		}
		if len(g.Images) == 1 {
			out = append(out, img)
		} else {
			out = append(out, g)
		}
	}
	return out
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte(escapable, s[i+1]) >= 0 {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Slug turns heading text into an anchor ID: "The link card" -> "the-link-card".
func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case r == ' ' || r == '-' || r == '_':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func assignIDs(blocks []Block, seen map[string]int) {
	for _, b := range blocks {
		switch b := b.(type) {
		case *Heading:
			id := Slug(PlainText(b.Inlines))
			if n := seen[id]; n > 0 {
				seen[id] = n + 1
				id += "-" + strconv.Itoa(n)
			} else {
				seen[id] = 1
			}
			b.ID = id
		case *Container:
			assignIDs(b.Blocks, seen)
		}
	}
}
