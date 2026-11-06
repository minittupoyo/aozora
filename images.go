package main

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// thumbs loads and keeps the thumbnails of the cards. Everything but the
// downloads runs on the main thread: get is called from the view, and the
// finished bitmaps arrive through post.
type thumbs struct {
	post   func(func()) // runs a function on the main thread and redraws
	client http.Client
	sem    chan struct{}
	bmp    map[string]*thumbEntry // nil value: failed
	busy   map[string]bool
	tick   uint64
}

// maxThumbs is how many thumbnails are kept: a few screens' worth. Older
// ones that are not in view are dropped, and fetched again if they come back.
const maxThumbs = 150

type thumbEntry struct {
	b        *ui.Bitmap
	used     uint64 // tick of the last frame it was in view
	released bool   // its CPU pixels are gone
}

func newThumbs(post func(func())) *thumbs {
	return &thumbs{
		post:   post,
		client: http.Client{Timeout: 20 * time.Second},
		sem:    make(chan struct{}, 6),
		bmp:    map[string]*thumbEntry{},
		busy:   map[string]bool{},
	}
}

// get returns the thumbnail at url, or nil while it loads or if it failed.
func (t *thumbs) get(url string) *ui.Bitmap {
	if url == "" {
		return nil
	}
	if th, ok := t.bmp[url]; ok {
		if th == nil {
			return nil
		}
		return th.b
	}
	if !t.busy[url] {
		t.busy[url] = true
		go t.fetch(url)
	}
	return nil
}

func (t *thumbs) fetch(url string) {
	t.sem <- struct{}{}
	defer func() { <-t.sem }()
	b := t.download(url)
	t.post(func() {
		delete(t.busy, url)
		if b == nil {
			t.bmp[url] = nil
			return
		}
		t.bmp[url] = &thumbEntry{b: b, used: t.tick}
		t.evict()
	})
}

func (t *thumbs) download(url string) *ui.Bitmap {
	// The API links the 1280x720 version, 3.7 MB once decoded. A card is 240 dp
	// wide, so the 480x270 one is as sharp as it can be shown at 2x.
	url = strings.Replace(url, "/xlarge/", "/small/", 1)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := t.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil
	}
	b, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return b
}

// peek returns the thumbnail at url if it is loaded, without asking for it.
func (t *thumbs) peek(url string) *ui.Bitmap {
	if th := t.bmp[url]; th != nil {
		return th.b
	}
	return nil
}

// touch says the thumbnail at url is in view. Once it has shown, its CPU
// pixels are dropped: the GPU keeps drawing it, and a bitmap that is not
// used for a while is dropped altogether by evict.
func (t *thumbs) touch(url string) {
	th := t.bmp[url]
	if th == nil {
		return
	}
	t.tick++
	th.used = t.tick
	if !th.released {
		th.released = true
		th.b.ReleasePixels()
	}
}

// evict drops the least recently shown thumbnails beyond maxThumbs.
func (t *thumbs) evict() {
	for len(t.bmp) > maxThumbs {
		oldest, at := "", ^uint64(0)
		for u, th := range t.bmp {
			if th != nil && th.used < at {
				oldest, at = u, th.used
			}
		}
		if oldest == "" {
			return
		}
		delete(t.bmp, oldest)
	}
}
