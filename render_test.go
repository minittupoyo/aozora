package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// fakeApp is an app without a window or a network, showing made-up content.
func fakeApp() *app {
	a := &app{tver: &Tver{}, epRanks: map[string][]Item{}, homeOK: true, started: true}
	a.thumbs = newThumbs(func(func()) {})
	a.player = newPlayer(a)
	items := func(n int, rank bool) []Item {
		var out []Item
		for i := 0; i < n; i++ {
			it := Item{ID: fmt.Sprint(i), Type: "episode", Title: fmt.Sprintf("サンプル番組のタイトルです その%d", i+1) + strings.Repeat("長い", (i%3)*12), Subtitle: "テレビ東京\n10月30日(木)放送分"}
			if rank {
				it.Rank = i + 1
			}
			if i%4 == 1 {
				it.Type = "series"
			}
			out = append(out, it)
		}
		return out
	}
	a.home = []Section{
		{Title: "ランキング", Items: items(12, true)},
		{Title: "新着", Items: items(12, false), Category: ""},
		{Title: "ドラマ 人気", Items: items(12, false), Category: "drama"},
		{Title: "バラエティ 人気", Items: items(12, false), Category: "variety"},
	}
	return a
}

func shot(t *testing.T, name string, w, h int, scale float32, view func(c *ui.Context)) {
	img := ui.Render(view, w, h, scale)
	dir := os.Getenv("SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestScreens(t *testing.T) {
	a := fakeApp()
	items := a.home[0].Items

	a.dest, a.cur = destHome, &screen{kind: scrHome, secs: a.home}
	shot(t, "home", 1200, 780, 1, a.view)

	a.dest, a.cur = destRanking, &screen{kind: scrRanking, rankKind: "series", rankGenre: "all",
		genres: []Genre{{"all", "総合"}, {"drama", "ドラマ"}, {"variety", "バラエティ"}, {"anime", "アニメ"}}, items: items}
	shot(t, "ranking", 1200, 780, 1, a.view)

	a.dest, a.cur = destCategories, &screen{kind: scrCategories, secs: a.home, catRank: items}
	shot(t, "categories", 1200, 780, 1, a.view)

	a.dest, a.cur = destSettings, &screen{kind: scrSettings, ytdlpOK: true, ytdlp: YtdlpInfo{Source: "managed", Path: `C:\Users\x\yt-dlp.exe`, Version: "2026.10.01", Managed: true}}
	shot(t, "settings", 1200, 780, 1, a.view)

	a.dest, a.cur = destHome, &screen{kind: scrHome, secs: a.home}
	a.busy = 1
	shot(t, "compact", 520, 780, 1, a.view)
}

func TestPlayer(t *testing.T) {
	a := fakeApp()
	a.dest, a.cur = destHome, &screen{kind: scrHome, secs: a.home}
	p := a.player
	p.on, p.title = true, "サンプル番組のタイトルです その1"
	p.lastActive = time.Now()

	p.st = PlayerState{Started: true, Paused: false, Time: 754, Duration: 2700, Volume: 0.8, Rate: 1, PipSupported: true}
	shot(t, "player-playing", 1200, 700, 1, a.view)

	p.st.Paused = true
	shot(t, "player-paused", 1200, 700, 1, a.view)

	p.st = PlayerState{Started: true, Paused: false, Pip: true, PipSupported: true, Time: 42, Duration: 2681, Volume: 1, Rate: 1}
	shot(t, "player-pip", 1200, 700, 1, a.view)

	p.st = PlayerState{Paused: true, Volume: 1, Rate: 1}
	p.preparing = true
	p.steps = []prepStep{{"yt-dlp で再生URLを取得", "ERROR: HTTP Error 403", 2}, {"Brightcove から再生URLを取得", "", 1}, {"ストリームを準備", "", 0}}
	shot(t, "player-preparing", 1200, 700, 1, a.view)

	p.preparing = false
	p.failure = "再生URLを取得できませんでした"
	shot(t, "player-failed", 1200, 700, 1, a.view)

	p.failure = ""
	p.mini = true
	p.st = PlayerState{Started: true, Paused: true, Time: 10, Duration: 100, Volume: 1, Rate: 1}
	shot(t, "player-mini", 1200, 700, 1, a.view)
}

func TestRipple(t *testing.T) {
	a := fakeApp()
	a.dest, a.cur = destHome, &screen{kind: scrHome, secs: a.home}
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Move(230, 250)
	tt.Press(230, 250)
	tt.Advance(200 * time.Millisecond)
	if dir := os.Getenv("SHOTS"); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "ripple.png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
}

func TestNavRipple(t *testing.T) {
	a := fakeApp()
	a.dest, a.cur = destHome, &screen{kind: scrHome, secs: a.home}
	tt := ui.NewTester(a.view, 1200, 780)
	tt.Move(30, 150)
	tt.Press(30, 150)
	tt.Advance(150 * time.Millisecond)
	if dir := os.Getenv("SHOTS"); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "navripple.png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
}
