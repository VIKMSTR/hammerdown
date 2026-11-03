package ui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // register decoders for DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

const maxRemoteImage = 20 << 20

// imageLoader resolves image paths relative to the document and caches
// decoded resources; remote images are fetched in the background.
type imageLoader struct {
	mu       sync.Mutex
	cache    map[string]*imageEntry
	client   *http.Client
	onUpdate func() // called on the UI thread when a remote image arrives
}

type imageEntry struct {
	res     fyne.Resource
	w, h    int
	err     error
	modTime time.Time
	loading bool
}

func newImageLoader() *imageLoader {
	return &imageLoader{
		cache:  map[string]*imageEntry{},
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func isRemote(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

// resolvePath maps a document-relative path to a file on disk. A bare filename
// that does not exist next to the document is looked up in incoming/.
func resolvePath(src, baseDir string) string {
	p := strings.TrimPrefix(src, "file://")
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	if filepath.IsAbs(p) {
		return p
	}
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	direct := filepath.Join(baseDir, filepath.FromSlash(p))
	if _, err := os.Stat(direct); err == nil || strings.Contains(p, "/") {
		return direct
	}
	incoming := filepath.Join(baseDir, "incoming", p)
	if _, err := os.Stat(incoming); err == nil {
		return incoming
	}
	return direct
}

// get returns the cached entry for src, loading local files synchronously and
// starting a background fetch for remote ones (returning nil while pending).
func (l *imageLoader) get(src, baseDir string) *imageEntry {
	if isRemote(src) {
		return l.getRemote(src)
	}
	path := resolvePath(src, baseDir)
	st, err := os.Stat(path)
	if err != nil {
		return &imageEntry{err: fmt.Errorf("not found: %s", path)}
	}
	l.mu.Lock()
	e := l.cache[path]
	l.mu.Unlock()
	if e != nil && e.modTime.Equal(st.ModTime()) {
		return e
	}
	data, err := os.ReadFile(path)
	e = &imageEntry{modTime: st.ModTime(), err: err}
	if err == nil {
		e.res = fyne.NewStaticResource(filepath.Base(path), data)
		e.w, e.h = imageSize(filepath.Base(path), data)
	}
	l.mu.Lock()
	l.cache[path] = e
	l.mu.Unlock()
	return e
}

func (l *imageLoader) getRemote(url string) *imageEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.cache[url]; e != nil {
		if e.loading {
			return nil
		}
		return e
	}
	l.cache[url] = &imageEntry{loading: true}
	go l.fetch(url)
	return nil
}

func (l *imageLoader) fetch(url string) {
	e := &imageEntry{}
	resp, err := l.client.Get(url)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("HTTP %s", resp.Status)
		}
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(resp.Body, maxRemoteImage))
	}
	if err == nil {
		name := filepath.Base(strings.SplitN(url, "?", 2)[0])
		if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "svg") && !strings.HasSuffix(name, ".svg") {
			name += ".svg"
		}
		e.res = fyne.NewStaticResource(name, data)
		e.w, e.h = imageSize(name, data)
	}
	e.err = err
	l.mu.Lock()
	l.cache[url] = e
	cb := l.onUpdate
	l.mu.Unlock()
	if cb != nil {
		fyne.Do(cb)
	}
}

var (
	svgWidthRe   = regexp.MustCompile(`<svg[^>]*\swidth="([\d.]+)`)
	svgHeightRe  = regexp.MustCompile(`<svg[^>]*\sheight="([\d.]+)`)
	svgViewBoxRe = regexp.MustCompile(`viewBox="[\d.\-]+[ ,]+[\d.\-]+[ ,]+([\d.]+)[ ,]+([\d.]+)"`)
)

// imageSize returns the pixel size of an image (0, 0 if unknown).
func imageSize(name string, data []byte) (int, int) {
	if strings.HasSuffix(strings.ToLower(name), ".svg") || bytes.Contains(data[:min(len(data), 512)], []byte("<svg")) {
		head := data[:min(len(data), 4096)]
		w, h := svgDim(svgWidthRe, head), svgDim(svgHeightRe, head)
		if w == 0 || h == 0 {
			if m := svgViewBoxRe.FindSubmatch(head); m != nil {
				vw, _ := strconv.ParseFloat(string(m[1]), 64)
				vh, _ := strconv.ParseFloat(string(m[2]), 64)
				w, h = int(vw), int(vh)
			}
		}
		return w, h
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func svgDim(re *regexp.Regexp, b []byte) int {
	m := re.FindSubmatch(b)
	if m == nil {
		return 0
	}
	f, _ := strconv.ParseFloat(string(m[1]), 64)
	return int(f)
}
