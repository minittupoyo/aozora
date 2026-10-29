package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The stream hosts reject requests whose Origin is not tver.jp, which the
// webview cannot fake. A loopback proxy fetches with TVer's Origin and rewrites
// the playlists so every segment goes through it too.

var (
	proxyOnce sync.Once
	proxyBase string
	proxyHTTP = &http.Client{Timeout: 60 * time.Second}
	uriAttrRe = regexp.MustCompile(`URI="([^"]+)"`)
)

// proxied returns the loopback URL that serves u.
func proxied(u string) (string, error) {
	var err error
	proxyOnce.Do(func() {
		var ln net.Listener
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return
		}
		proxyBase = "http://" + ln.Addr().String() + "/p?u="
		go http.Serve(ln, http.HandlerFunc(serveProxy))
	})
	if err != nil {
		return "", err
	}
	return proxyBase + url.QueryEscape(u), nil
}

func serveProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == http.MethodOptions {
		return
	}
	target := r.URL.Query().Get("u")
	tu, err := url.Parse(target)
	if err != nil || (tu.Scheme != "https" && tu.Scheme != "http") {
		http.Error(w, "bad url", http.StatusBadRequest)
		return
	}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", "https://tver.jp")
	req.Header.Set("Referer", "https://tver.jp/")
	if rg := r.Header.Get("Range"); rg != "" {
		req.Header.Set("Range", rg)
	}
	res, err := proxyHTTP.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()

	isPlaylist := strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "mpegurl") ||
		strings.HasSuffix(tu.Path, ".m3u8")
	if !isPlaylist {
		for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := res.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
		return
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(res.StatusCode)
	w.Write(rewritePlaylist(body, tu))
}

func rewritePlaylist(body []byte, base *url.URL) []byte {
	abs := func(ref string) string {
		u, err := base.Parse(ref)
		if err != nil {
			return ref
		}
		p, err := proxied(u.String())
		if err != nil {
			return ref
		}
		return p
	}
	var out bytes.Buffer
	for _, line := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
			line = uriAttrRe.ReplaceAllStringFunc(line, func(m string) string {
				return `URI="` + abs(uriAttrRe.FindStringSubmatch(m)[1]) + `"`
			})
		case strings.TrimSpace(line) != "":
			line = abs(strings.TrimSpace(line))
		}
		out.WriteString(line + "\n")
	}
	return out.Bytes()
}
