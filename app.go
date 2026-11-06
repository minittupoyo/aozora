package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// dest is a top-level destination of the navigation rail.
type dest int

const (
	destHome dest = iota
	destCategories
	destRanking
	destSearch
	destSettings
)

type screenKind int

const (
	scrHome screenKind = iota
	scrCategories
	scrRanking
	scrSearch
	scrSettings
	scrSeries
)

// screen is one page of the app and everything it keeps while it is open.
type screen struct {
	kind    screenKind
	title   string
	loading bool
	err     string
	secs    []Section
	scroll  ui.ScrollState

	// series and search: which season is shown, -1 for all
	season int

	// categories
	cat     int
	catRank []Item

	// ranking
	rankKind    string // "series" or "episode"
	rankGenre   string
	genres      []Genre
	items       []Item
	rankLoading bool
	rankSeq     int

	// settings
	ytdlp     YtdlpInfo
	ytdlpOK   bool
	ytdlpBusy bool
}

// category is a genre of the home page.
var categories = []struct{ id, label string }{
	{"drama", "ドラマ"},
	{"variety", "バラエティ"},
	{"anime", "アニメ"},
	{"news", "報道・ニュース"},
	{"sports", "スポーツ"},
	{"others", "その他"},
}

// rankGenreOf is the genre of TVer's episode ranking that matches a category.
var rankGenreOf = map[string]string{
	"drama":   "drama",
	"variety": "variety",
	"anime":   "anime",
	"news":    "news_documentary",
	"sports":  "sports",
	"others":  "other",
}

type app struct {
	win    *mygo.Window
	tver   *Tver
	thumbs *thumbs

	dest  dest
	cur   *screen
	back  []*screen
	query string
	busy  int // loads under way, for the progress bar

	home        []Section
	homeOK      bool
	epRanks     map[string][]Item
	player      *player
	searchID    ui.Handle
	focusSearch bool
	started     bool
}

func newApp(win *mygo.Window) *app {
	a := &app{win: win, tver: &Tver{}, epRanks: map[string][]Item{}}
	a.thumbs = newThumbs(a.post)
	a.player = newPlayer(a)
	return a
}

// post runs fn on the main thread and redraws.
func (a *app) post(fn func()) { a.win.Update(fn) }

// kick redraws shortly after, once the layout of new content is known, so
// that cards in view can ask for their thumbnails.
func (a *app) kick() {
	time.AfterFunc(120*time.Millisecond, func() { a.post(func() {}) })
}

// load runs fn off the main thread; done runs on it with fn's error.
func (a *app) load(s *screen, fn func() error, done func()) {
	a.busy++
	s.loading = true
	go func() {
		err := fn()
		a.post(func() {
			a.busy--
			s.loading = false
			s.err = ""
			if err != nil {
				s.err = firstLine(err)
			}
			if done != nil {
				done()
			}
			a.kick()
		})
	}()
}

func firstLine(err error) string {
	s := strings.TrimPrefix(err.Error(), "CallError: ")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// ---- navigation ----

func (a *app) open(d dest) {
	a.dest = d
	a.back = nil
	switch d {
	case destHome:
		a.query = ""
		a.showHome()
	case destCategories:
		a.query = ""
		a.showCategories()
	case destRanking:
		a.query = ""
		a.showRanking()
	case destSearch:
		a.focusSearch = true
	case destSettings:
		a.query = ""
		a.showSettings()
	}
}

func (a *app) goBack() bool {
	n := len(a.back)
	if n == 0 {
		return false
	}
	a.cur = a.back[n-1]
	a.back = a.back[:n-1]
	return true
}

func (a *app) showHome() {
	s := &screen{kind: scrHome, title: "ホーム"}
	a.cur = s
	var secs []Section
	a.load(s, func() (err error) { secs, err = a.tver.Home(); return }, func() {
		if s.err == "" {
			s.secs = secs
			a.home, a.homeOK = secs, true
		}
	})
}

// homeSections returns the home page, fetching it once.
func (a *app) homeSections() ([]Section, error) {
	if a.homeOK {
		return a.home, nil
	}
	return a.tver.Home()
}

func (a *app) showCategories() {
	s := &screen{kind: scrCategories, title: "カテゴリ"}
	a.cur = s
	var secs []Section
	a.load(s, func() (err error) { secs, err = a.homeSections(); return }, func() {
		if s.err != "" {
			return
		}
		a.home, a.homeOK = secs, true
		s.secs = secs
		a.selectCategory(s, 0)
	})
}

// groups are the categories that have rows on the home page.
func (a *app) groups(s *screen) []struct {
	id, label string
	secs      []Section
} {
	var out []struct {
		id, label string
		secs      []Section
	}
	for _, cat := range categories {
		var secs []Section
		for _, sec := range s.secs {
			// the home page only carries a top-10 ranking; the full one is added below
			if sec.Category == cat.id && !strings.Contains(sec.Title, "ランキング") {
				secs = append(secs, sec)
			}
		}
		if len(secs) > 0 {
			out = append(out, struct {
				id, label string
				secs      []Section
			}{cat.id, cat.label, secs})
		}
	}
	return out
}

func (a *app) selectCategory(s *screen, i int) {
	s.cat = i
	s.catRank = nil
	s.scroll.Y = 0
	g := a.groups(s)
	if i >= len(g) {
		return
	}
	genre := rankGenreOf[g[i].id]
	if genre == "" {
		genre = "all"
	}
	if items, ok := a.epRanks[genre]; ok {
		s.catRank = items
		return
	}
	go func() {
		items, err := a.tver.Ranking("episode", genre)
		if err != nil {
			return
		}
		a.post(func() {
			a.epRanks[genre] = items
			if s.cat == i {
				s.catRank = items
			}
			a.kick()
		})
	}()
}

func (a *app) showRanking() {
	s := &screen{kind: scrRanking, title: "ランキング", rankKind: "series", rankGenre: "all"}
	a.cur = s
	a.loadRankingGenres(s)
}

func (a *app) loadRankingGenres(s *screen) {
	var genres []Genre
	kind := s.rankKind
	a.load(s, func() (err error) { genres, err = a.tver.RankingGenres(kind); return }, func() {
		if s.err != "" {
			return
		}
		s.genres = genres
		ok := false
		for _, g := range genres {
			ok = ok || g.ID == s.rankGenre
		}
		if !ok && len(genres) > 0 {
			s.rankGenre = genres[0].ID
		}
		a.loadRanking(s)
	})
}

func (a *app) loadRanking(s *screen) {
	s.rankSeq++
	seq := s.rankSeq
	kind, genre := s.rankKind, s.rankGenre
	s.rankLoading = true
	a.busy++
	go func() {
		items, err := a.tver.Ranking(kind, genre)
		a.post(func() {
			a.busy--
			if seq != s.rankSeq {
				return // a newer genre was picked meanwhile
			}
			s.rankLoading = false
			if err != nil {
				s.err = firstLine(err)
				return
			}
			s.err = ""
			s.items = items
			s.scroll.Y = 0
			a.kick()
		})
	}()
}

func (a *app) runSearch() {
	q := strings.TrimSpace(a.query)
	if q == "" {
		a.open(destHome)
		return
	}
	a.dest = destSearch
	a.back = nil
	s := &screen{kind: scrSearch, title: fmt.Sprintf("「%s」の検索結果", q), season: -1}
	a.cur = s
	var secs []Section
	a.load(s, func() (err error) { secs, err = a.tver.Search(q); return }, func() {
		if s.err == "" {
			s.secs = secs
		}
	})
}

func (a *app) openSeries(it Item) {
	prev := a.cur
	s := &screen{kind: scrSeries, title: it.Title}
	var secs []Section
	a.busy++
	go func() {
		var err error
		secs, err = a.tver.SeriesEpisodes(it.ID)
		a.post(func() {
			a.busy--
			if err != nil {
				a.toast("エラー: " + firstLine(err))
				return
			}
			if len(secs) == 0 {
				a.toast("エピソードがありません")
				return
			}
			s.secs = secs
			a.back = append(a.back, prev)
			a.cur = s
			a.kick()
		})
	}()
}

func (a *app) openItem(it Item) {
	if it.Type == "series" {
		a.openSeries(it)
		return
	}
	a.player.open(it.ID, it.Title)
}

func (a *app) showSettings() {
	s := &screen{kind: scrSettings, title: "設定"}
	a.cur = s
	go func() {
		info := a.tver.YtdlpStatus()
		a.post(func() { s.ytdlp, s.ytdlpOK = info, true })
	}()
}

func (a *app) runYtdlp(s *screen, label string, fn func() (YtdlpInfo, error)) {
	s.ytdlpBusy = true
	go func() {
		info, err := fn()
		a.post(func() {
			s.ytdlpBusy = false
			if err != nil {
				a.toast("エラー: " + firstLine(err))
				return
			}
			s.ytdlp = info
			a.toast(label + "が完了しました")
		})
	}()
}

// toasts waiting for a view to show them (a toast needs the Context).
func (a *app) toast(msg string) { pendingToasts = append(pendingToasts, msg) }

var pendingToasts []string
