package main

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"
)

const (
	railWidth  = 80
	contentMax = 1440
	pageGutter = 16
	cardMinW   = 240
	cardGap    = 8
	rowCardW   = 240
)

// ---- the shell ----

func (a *app) view(c *ui.Context) {
	p := pal(c)
	for _, m := range pendingToasts {
		c.Toast(m)
	}
	pendingToasts = nil

	if a.player.visible() && !a.player.mini {
		a.player.fullView(c, p)
		return
	}

	w, _ := c.Size()
	compact := w < 600
	a.shortcuts(c)

	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(p.surface).Children(func() {
		if !compact {
			a.rail(c, p)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			a.appBar(c, p)
			avail := min(w-pickf(compact, 0, railWidth), contentMax) - 2*pageGutter
			a.content(c, p, avail)
			if compact {
				a.bar(c, p)
			}
		})
		if a.player.visible() {
			a.player.miniView(c, p)
		}
	})
}

func (a *app) shortcuts(c *ui.Context) {
	typing := a.searchID.Focused(c)
	if c.Shortcut(ui.Alt, ui.KeyLeft) || (!typing && c.Shortcut(0, ui.KeyEscape)) {
		a.goBack()
	}
	if c.Shortcut(ui.Ctrl, ui.KeyF) {
		a.searchID.Focus(c)
	}
	if a.focusSearch {
		a.searchID.Focus(c)
		a.focusSearch = false
	}
}

// ---- navigation ----

var navItems = []struct {
	d     dest
	label string
	icon  *ui.SVG
}{
	{destHome, "ホーム", icHome},
	{destCategories, "カテゴリ", icCategories},
	{destRanking, "ランキング", icRanking},
	{destSearch, "検索", icSearch},
	{destSettings, "設定", icSettings},
}

// navItem is one destination: an icon in a pill that fills in when it is
// chosen, over its label.
func (a *app) navItem(c *ui.Context, p *palette, i int) {
	it := navItems[i]
	selected := a.dest == it.d
	b := ui.ButtonBase(c.Key(i))
	hov, prs := b.Hovered(), b.Pressed()
	// the ripple is the indicator's: 56 x 32 under the item's top padding
	rippleIn(b, pickcol(selected, p.onSecondaryContainer, p.onSurfaceVariant), 16, false, func(r ui.Rect) ui.Rect {
		return ui.Rect{X: r.X + (r.W-56)/2, Y: r.Y + 6, W: 56, H: 32}
	})
	b.Cursor(ui.CursorPointer).Label(it.label).Background(ui.Transparent)
	b.Padding(6, 0).Radius(shapeM).Children(func() {
		ui.Column(c).Grow(1).AlignItems(ui.Center).Gap(4).Children(func() {
			pill := ui.Box(c).Height(32).Center()
			w := pill.AnimateTo("w", pickf(selected, 56, 32), spatial)
			bg := ui.Transparent
			if selected {
				bg = p.secondaryContainer
			}
			if hov || prs {
				if bg.A == 0 {
					bg = p.onSurface.Alpha(0.08)
				} else {
					bg = bg.Mix(p.onSurface, 0.08)
				}
			}
			fg := pickcol(selected, p.onSecondaryContainer, p.onSurfaceVariant)
			pill.Width(56).Radius(16).Background(bg).TextColor(fg)
			_ = w
			pill.Children(func() { ui.Icon(c, it.icon).FontSize(24) })
			text(c, it.label, tsLabelMedium, pickcol(selected, p.onSurface, p.onSurfaceVariant)).
				FontWeight(pickw(selected, 700, 500)).SingleLine()
		})
	})
	if b.Clicked() {
		a.open(it.d)
	}
}

func pickcol(ok bool, a, b ui.Color) ui.Color {
	if ok {
		return a
	}
	return b
}

func pickw(ok bool, a, b int) int {
	if ok {
		return a
	}
	return b
}

func (a *app) rail(c *ui.Context, p *palette) {
	ui.Column(c).Width(railWidth).Padding(44, 8, 8).Gap(12).AlignItems(ui.Stretch).Background(p.surface).Children(func() {
		for i := range navItems {
			a.navItem(c, p, i)
		}
	})
}

func (a *app) bar(c *ui.Context, p *palette) {
	ui.Row(c).Height(80).Padding(12, 8, 16).AlignItems(ui.Stretch).Background(p.surfaceContainer).Children(func() {
		for i := range navItems {
			ui.Box(c.Key(i)).Grow(1).Children(func() { a.navItem(c, p, i) })
		}
	})
}

// ---- the top app bar ----

func (a *app) appBar(c *ui.Context, p *palette) {
	ui.Column(c).Background(p.surface).Children(func() {
		ui.Row(c).Height(72).Padding(0, 16).Gap(16).Children(func() {
			brand := ui.ButtonBase(c.Key("brand")).Height(48).Padding(0, 12).Gap(8).Center().Radius(24).Cursor(ui.CursorPointer).Label("ホーム")
			brand.Background(layer(ui.Transparent.Alpha(0), p.primary, brand.Hovered(), brand.Pressed()).Mix(p.primary.Alpha(0.08), pickf(brand.Hovered(), 1, 0))).TextColor(p.primary)
			brand.Children(func() {
				ui.Icon(c, icPlay).FontSize(28)
				text(c, "Aozora", tsTitleLarge, p.primary).FontWeight(700)
			})
			if brand.Clicked() {
				a.open(destHome)
			}

			ui.Spacer(c)
			a.searchBar(c, p)
			ui.Spacer(c)
			ui.Box(c).Width(96)
		})
		if a.busy > 0 || a.player.loading() {
			loadingBar(c, p)
		} else {
			ui.Box(c).Height(8)
		}
	})
}

// searchBar is Material 3's search bar: a pill with the query in it.
func (a *app) searchBar(c *ui.Context, p *palette) {
	field := ui.Row(c.Key("search")).Height(48).Width(560).MaxWidth(560).Shrink(1).MinWidth(120).Padding(0, 4, 0, 16).Gap(12).Center()
	hov := field.Hovered()
	focused := a.searchID.FocusWithin(c)
	bg := layer(p.surfaceContainerHigh, p.onSurface, hov, false)
	r := field.AnimateTo("r", 24, spatialFast)
	field.Radius(r).Background(bg)
	if focused {
		field.Border(2, p.primary)
	}
	field.Children(func() {
		ui.Icon(c, icSearch).FontSize(24).TextColor(p.onSurfaceVariant)
		in := ui.TextInputBase(c.Key("q"), &a.query).Bind(&a.searchID).Grow(1).Placeholder("番組を検索").
			FontSize(tsBodyLarge.size).TextColor(p.onSurface)
		if in.Submitted() {
			a.runSearch()
		}
		if a.query != "" {
			if iconButton(c, p, "clear", "クリア", icClose, ui.Color{}, false).Clicked() {
				a.query = ""
				a.searchID.Focus(c)
			}
		}
	})
}

// ---- the page ----

func (a *app) content(c *ui.Context, p *palette, avail float32) {
	s := a.cur
	if s == nil {
		if a.dest == destHome && !a.started {
			a.started = true
			a.showHome()
		}
		return
	}
	ui.Scroll(c.Key(s)).TrackScroll(&s.scroll).Grow(1).Children(func() {
		ui.Column(c).WidthPercent(100).MaxWidth(contentMax).Margin(0, ui.Auto).Padding(8, pageGutter, 32).Gap(0).Children(func() {
			switch {
			case s.err != "" && !s.hasContent():
				emptyState(c, p, "読み込めませんでした\n"+s.err)
			case s.loading && !s.hasContent():
				ui.Column(c).Center().Padding(120, 0).Children(func() { loadingIndicator(c, p, 64) })
			default:
				a.page(c, p, s, avail)
			}
		})
	})
}

func (s *screen) hasContent() bool {
	switch s.kind {
	case scrRanking:
		return len(s.genres) > 0
	case scrSettings:
		return true
	}
	return len(s.secs) > 0
}

func (a *app) page(c *ui.Context, p *palette, s *screen, avail float32) {
	switch s.kind {
	case scrHome:
		if len(s.secs) == 0 {
			emptyState(c, p, "コンテンツがありません")
			return
		}
		a.sections(c, p, "home", s.secs)
	case scrCategories:
		a.categoriesPage(c, p, s)
	case scrRanking:
		a.rankingPage(c, p, s, avail)
	case scrSearch:
		if len(s.secs) == 0 {
			emptyState(c, p, "見つかりませんでした")
			return
		}
		a.seasonsPage(c, p, s, avail)
	case scrSeries:
		a.seasonsPage(c, p, s, avail)
	case scrSettings:
		a.settingsPage(c, p, s)
	}
}

// pageHeader is the title of a page, with a back button when there is a page to go back to.
func (a *app) pageHeader(c *ui.Context, p *palette, title string) {
	ui.Row(c).Gap(8).Margin(16, 8, 12).Children(func() {
		if len(a.back) > 0 {
			if iconButton(c, p, "back", "戻る", icBack, p.onSurface, false).Clicked() {
				a.goBack()
			}
		}
		text(c, title, tsHeadlineSmall, p.onSurface).Grow(1).MaxLines(2)
	})
}

func sectionTitle(c *ui.Context, p *palette, s string) {
	text(c, s, tsTitleLarge, p.onSurface).FontWeight(600).Margin(24, 8, 12)
}

// sections lays out titled rows of cards that scroll sideways.
func (a *app) sections(c *ui.Context, p *palette, ns string, secs []Section) {
	for si, sec := range secs {
		ui.Column(c.Key(fmt.Sprint(ns, si))).Children(func() {
			sectionTitle(c, p, sec.Title)
			ui.ScrollHorizontal(c.Key("row")).Gap(cardGap).Padding(0, 8, 12).Children(func() {
				for i, it := range sec.Items {
					a.card(c, p, it, i, rowCardW).Shrink(0)
				}
			})
		})
	}
}

// grid lays cards out in as many equal columns as fit.
func (a *app) grid(c *ui.Context, p *palette, items []Item, avail float32) {
	cols := max(1, int((avail-16+cardGap)/(cardMinW+cardGap)))
	w := (avail - 16 - float32(cols-1)*cardGap) / float32(cols)
	ui.Row(c).Wrap().Gap(cardGap).Padding(0, 8).Children(func() {
		for i, it := range items {
			a.card(c, p, it, i, floor(w))
		}
	})
}

func floor(v float32) float32 { return float32(int(v)) }

// card is a content card: a thumbnail over a title and a subtitle. On press
// its corners tighten, and the thumbnail rounds with them.
func (a *app) card(c *ui.Context, p *palette, it Item, i int, width float32) ui.Element {
	// Material 3 elevated card: 12 dp corners, surface container low, level 1
	// elevation (level 2 on hover), media edge to edge, 16 dp around the text.
	b := ui.ButtonBase(c.Key(i))
	hov, prs := b.Hovered(), b.Pressed()
	r := float32(shapeM)
	elev := b.AnimateTo("e", pickf(hov && !prs, 1, 0), spatialFast)
	bg := layer(p.surfaceContainerLow, p.onSurface, hov, prs)
	paintRipple := ripple(b, p.onSurface, r, true)
	b.Width(width).Padding(0).Radius(r).Background(bg).Clip().Cursor(ui.CursorPointer).Label(it.Title)
	shadow := pickcol(p.dark, ui.RGBA(0, 0, 0, 0.5), ui.RGBA(0, 0, 0, 0.22))
	b.Shadow(0, 1+elev, 2+2*elev, 0, shadow)
	b.Children(func() {
		defer rippleOver(c, paintRipple)
		ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			media := ui.Box(c.Key("media")).AspectRatio(16.0 / 9).Background(p.surfaceContainerHigh)
			bmp := a.thumbs.peek(it.Thumbnail)
			if media.InView() {
				if bmp == nil {
					a.thumbs.get(it.Thumbnail)
				} else {
					a.thumbs.touch(it.Thumbnail)
				}
			}
			media.Children(func() {
				if bmp != nil {
					img := ui.Image(c, bmp).Fill().Fit(ui.Cover)
					img.Opacity(img.AnimateTo("fade", 1, effects))
				}
				switch {
				case it.Rank > 0:
					bg, fg := p.inverseSurface, p.inverseOnSurface
					if it.Rank <= 3 {
						bg, fg = p.primary, p.onPrimary
					}
					ui.Box(c).Absolute().Left(8).Top(8).MinWidth(28).Height(28).Padding(0, 6).Center().Radius(shapeS).Background(bg).Children(func() {
						text(c, strconv.Itoa(it.Rank), tsLabelLarge, fg).FontWeight(700)
					})
				case it.Type == "series":
					ui.Box(c).Absolute().Left(8).Top(8).Padding(2, 8).Radius(shapeS).Background(p.primaryContainer).Children(func() {
						text(c, "シリーズ", tsLabelMedium, p.onPrimaryContainer)
					})
				}
			})
			// the text block has a fixed height, so every card is as tall as the others
			ui.Column(c).Padding(12, 16, 16).Gap(4).Children(func() {
				ui.Box(c.Key("t")).Height(40).Children(func() {
					text(c, it.Title, tsTitleSmall, p.onSurface).MaxLines(2)
				})
				ui.Box(c.Key("s")).Height(32).Children(func() {
					if it.Subtitle != "" {
						text(c, it.Subtitle, tsLabelMedium, p.onSurfaceVariant).FontWeight(400).MaxLines(2)
					}
				})
			})
		})
	})
	if b.Clicked() {
		a.openItem(it)
	}
	return b
}

// ---- categories ----

func (a *app) categoriesPage(c *ui.Context, p *palette, s *screen) {
	g := a.groups(s)
	if len(g) == 0 {
		emptyState(c, p, "カテゴリを取得できませんでした")
		return
	}
	s.cat = min(s.cat, len(g)-1)
	a.pageHeader(c, p, "カテゴリ")
	ui.Row(c).Wrap().Gap(8).Margin(0, 8, 16).Children(func() {
		for i, gr := range g {
			if filterChip(c, p, gr.id, fmt.Sprintf("%s  %d", gr.label, len(gr.secs)), i == s.cat).Clicked() && i != s.cat {
				a.selectCategory(s, i)
			}
		}
	})
	if len(s.catRank) > 0 {
		a.sections(c, p, "rank", []Section{{Title: "ランキング", Items: s.catRank}})
	}
	a.sections(c, p, fmt.Sprint("cat", s.cat), g[s.cat].secs)
}

// ---- ranking ----

func (a *app) rankingPage(c *ui.Context, p *palette, s *screen, avail float32) {
	a.pageHeader(c, p, "ランキング")
	kinds := []string{"series", "episode"}
	sel := 0
	if s.rankKind == "episode" {
		sel = 1
	}
	ui.Row(c).Margin(0, 8, 12).Children(func() {
		if i := connectedGroup(c, p, "kind", []string{"番組ランキング", "エピソードランキング"}, sel); i >= 0 && i != sel {
			s.rankKind = kinds[i]
			s.items = nil
			s.genres = nil
			a.loadRankingGenres(s)
		}
	})
	ui.Row(c).Wrap().Gap(8).Margin(0, 8, 16).Children(func() {
		for _, gr := range s.genres {
			if filterChip(c, p, gr.ID, gr.Title, gr.ID == s.rankGenre).Clicked() && gr.ID != s.rankGenre {
				s.rankGenre = gr.ID
				s.items = nil
				a.loadRanking(s)
			}
		}
	})
	switch {
	case len(s.items) > 0:
		a.grid(c, p, s.items, avail)
	case s.rankLoading:
		ui.Column(c).Center().Padding(80, 0).Children(func() { loadingIndicator(c, p, 56) })
	case s.err != "":
		emptyState(c, p, s.err)
	default:
		text(c, "ランキングがありません", tsBodyMedium, p.onSurfaceVariant).Margin(24, 8)
	}
}

// ---- seasons (series and search) ----

func (a *app) seasonsPage(c *ui.Context, p *palette, s *screen, avail float32) {
	a.pageHeader(c, p, s.title)
	if s.season >= len(s.secs) {
		s.season = -1
	}
	ui.Row(c).Wrap().Gap(8).Margin(0, 8, 16).Children(func() {
		if filterChip(c, p, "all", "すべて", s.season == -1).Clicked() {
			s.season = -1
		}
		for k, sec := range s.secs {
			if filterChip(c, p, k, sec.Title, s.season == k).Clicked() {
				s.season = k
			}
		}
	})
	for k, sec := range s.secs {
		if s.season != -1 && s.season != k {
			continue
		}
		ui.Column(c.Key(k)).Children(func() {
			text(c, fmt.Sprintf("%s  %d", sec.Title, len(sec.Items)), tsTitleMedium, p.onSurface).Margin(8, 8, 12)
			a.grid(c, p, sec.Items, avail)
		})
	}
}

// ---- settings ----

var sourceLabel = map[string]string{
	"managed": "アプリ内のコピーを使用中",
	"path":    "PATH 上の yt-dlp を使用中",
	"none":    "見つかりません",
}

func (a *app) settingsPage(c *ui.Context, p *palette, s *screen) {
	a.pageHeader(c, p, "設定")
	ui.Column(c).MaxWidth(640).Margin(0, 8).Padding(24).Radius(shapeXL).Background(p.surfaceContainerHigh).Children(func() {
		text(c, "yt-dlp", tsTitleLarge, p.onSurface)
		text(c, "再生URLの取得に使います。アプリ専用のコピーをダウンロードすると、PATH にあるものより優先して使います。",
			tsBodyMedium, p.onSurfaceVariant).Margin(4, 0, 16)
		if !s.ytdlpOK {
			text(c, "確認中…", tsBodyMedium, p.onSurface)
			return
		}
		info := s.ytdlp
		src := sourceLabel[info.Source]
		if src == "" {
			src = info.Source
		}
		row := func(label, value string) {
			ui.Divider(c).Background(p.outlineVariant)
			ui.Row(c).Padding(10, 0).Gap(8).AlignItems(ui.Start).Children(func() {
				text(c, label, tsLabelLarge, p.onSurfaceVariant).Width(96).Shrink(0)
				text(c, orDash(value), tsBodyMedium, p.onSurface).Grow(1).Selectable()
			})
		}
		row("状態", src)
		row("バージョン", info.Version)
		row("場所", info.Path)
		if s.ytdlpBusy {
			ui.Box(c).Margin(16, 0, 0).Children(func() { loadingBar(c, p) })
		}
		ui.Row(c).Wrap().Gap(8).Margin(16, 0, 0).Children(func() {
			label := pickstr(info.Managed, "最新版に更新", "ダウンロード")
			install := button(c, p, "install", label, pickic(info.Managed, icRefresh, icDownload), btnFilled).Disabled(s.ytdlpBusy)
			if install.Clicked() {
				a.runYtdlp(s, label, a.tver.YtdlpInstall)
			}
			if info.Managed {
				rm := button(c, p, "remove", "アプリ内のコピーを削除", icDelete, btnText).Disabled(s.ytdlpBusy)
				if rm.Clicked() {
					a.runYtdlp(s, "削除", a.tver.YtdlpRemove)
				}
			}
		})
	})
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func pickstr(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}

func pickic(ok bool, a, b *ui.SVG) *ui.SVG {
	if ok {
		return a
	}
	return b
}
