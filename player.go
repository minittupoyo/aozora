package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The video itself plays in a web view (the toolkit has no video codecs yet):
// the page is a <video> and hls.js, nothing else. Everything the user sees
// and touches is native, drawn over it, and drives the page with events.

// PlayerCommand is sent to the page.
type PlayerCommand struct {
	// Op is "load", "play", "pause", "seek", "skip", "volume", "mute", "rate",
	// "pip" or "stop". Value is the seconds, volume (0..1) or rate it needs.
	Op    string  `json:"op"`
	URL   string  `json:"url,omitempty"`
	Value float64 `json:"value,omitempty"`
}

// PlayerCmd carries commands to the player page.
var PlayerCmd = mygo.NewEvent[PlayerCommand]("player:cmd")

// PlayerState is what the page reports about the video.
type PlayerState struct {
	Started      bool    `json:"started"` // playback began: the manifest loaded and a frame played
	Paused       bool    `json:"paused"`
	Buffering    bool    `json:"buffering"`
	Time         float64 `json:"time"`
	Duration     float64 `json:"duration"`
	Volume       float64 `json:"volume"`
	Muted        bool    `json:"muted"`
	Rate         float64 `json:"rate"`
	PipSupported bool    `json:"pipSupported"`
	Pip          bool    `json:"pip"`
	// Error is a fatal playback error; empty when all is well.
	Error string `json:"error,omitempty"`
}

// PlayerBridge is what the page calls.
type PlayerBridge struct{}

// Report tells the app how the video is doing.
func (PlayerBridge) Report(s PlayerState) {
	if theApp == nil {
		return
	}
	theApp.post(func() { theApp.player.report(s) })
}

// theApp is the app of the one window, for the bound bridge.
var theApp *app

type prepStep struct {
	label, detail string
	state         int // 0 running, 1 ok, 2 failed
}

type player struct {
	a    *app
	web  *mygo.WebView
	on   bool // a video is open
	mini bool
	full bool // the window is full screen

	id, title string
	gen       int // the open of this video, so late results of an earlier one are dropped

	preparing bool
	steps     []prepStep
	failure   string // shown in place of the steps

	st PlayerState

	lastActive time.Time
	lastPtr    ui.Point
	volOpen    bool
}

func newPlayer(a *app) *player {
	return &player{a: a, st: PlayerState{Paused: true, Volume: 1, Rate: 1}}
}

func (p *player) visible() bool { return p.on }
func (p *player) loading() bool { return p.on && (p.preparing || (p.st.Buffering && !p.st.Paused)) }

func (p *player) send(op string, v float64) {
	if p.web != nil {
		PlayerCmd.EmitPage(p.web.Page(), PlayerCommand{Op: op, Value: v})
	}
}

// open starts the video of an episode.
func (p *player) open(id, title string) {
	if p.web == nil {
		w, err := p.a.win.NewWebView(mygo.WebViewOptions{
			URL:             "/",
			BackgroundColor: "#000000",
			Page:            mygo.PageOptions{Autoplay: mygo.AutoplayAllow},
		})
		if err != nil {
			p.a.toast("プレイヤーを開けません: " + firstLine(err))
			return
		}
		p.web = w
	}
	p.mini = false
	p.on = true
	p.id, p.title = id, title
	p.st = PlayerState{Paused: true, Volume: p.st.Volume, Muted: p.st.Muted, Rate: 1, PipSupported: p.st.PipSupported}
	p.wake()
	p.start()
}

func (p *player) close() {
	if p.full {
		p.setFull(false)
	}
	p.gen++
	p.on = false
	p.preparing = false
	p.send("stop", 0)
}

func (p *player) setFull(on bool) {
	p.full = on
	p.a.win.SetFullScreen(on)
}

// start resolves the stream (yt-dlp first, Brightcove if that fails), puts it
// behind the loopback proxy and hands it to the page.
func (p *player) start() {
	p.gen++
	gen, id := p.gen, p.id
	p.preparing = true
	p.failure = ""
	p.steps = nil
	p.st.Started, p.st.Error = false, ""
	post := func(fn func()) {
		p.a.post(func() {
			if p.gen == gen && p.on {
				fn()
			}
		})
	}
	go func() {
		n := 0
		add := func(label string) int {
			i := n
			n++
			post(func() { p.steps = append(p.steps, prepStep{label: label}) })
			return i
		}
		set := func(i, state int, detail string) {
			post(func() { p.steps[i].state, p.steps[i].detail = state, detail })
		}
		fail := func(msg string) {
			post(func() { p.preparing, p.failure = false, msg })
		}

		var s Stream
		y := add("yt-dlp で再生URLを取得")
		var err error
		if s, err = p.a.tver.StreamYtdlp(id); err != nil {
			set(y, 2, firstLine(err))
			d := add("Brightcove から再生URLを取得")
			if s, err = p.a.tver.StreamDirect(id); err != nil {
				set(d, 2, firstLine(err))
				fail("再生URLを取得できませんでした")
				return
			}
			set(d, 1, "")
		} else {
			set(y, 1, "")
		}
		pr := add("ストリームを準備")
		url, err := p.a.tver.Proxy(s.URL)
		if err != nil {
			set(pr, 2, firstLine(err))
			fail(firstLine(err))
			return
		}
		set(pr, 1, "")
		m := add("マニフェストを読み込み")
		post(func() {
			p.preparing = false
			p.steps[m].state = 0
			PlayerCmd.EmitPage(p.web.Page(), PlayerCommand{Op: "load", URL: url})
		})
	}()
}

// report takes the page's state.
func (p *player) report(s PlayerState) {
	if !p.on {
		return
	}
	wasPaused := p.st.Paused
	p.st = s
	if s.Error != "" {
		p.failure = s.Error
	}
	if s.Started {
		p.steps = nil
	}
	if s.Paused != wasPaused {
		p.wake()
	}
}

// ---- commands ----

func (p *player) toggle() {
	if p.st.Paused {
		p.send("play", 0)
	} else {
		p.send("pause", 0)
	}
	p.st.Paused = !p.st.Paused
	p.wake()
}

func (p *player) skip(d float64) {
	if p.st.Duration <= 0 {
		return
	}
	p.st.Time = math.Min(math.Max(p.st.Time+d, 0), p.st.Duration)
	p.send("skip", d)
	p.wake()
}

func (p *player) seekTo(t float64) {
	p.st.Time = t
	p.send("seek", t)
	p.wake()
}

func (p *player) setVolume(v float64) {
	v = math.Min(1, math.Max(0, v))
	p.st.Volume, p.st.Muted = v, v == 0
	p.send("volume", v)
	p.wake()
}

func (p *player) toggleMute() {
	p.st.Muted = !p.st.Muted
	p.send("mute", pickf64(p.st.Muted, 1, 0))
	p.wake()
}

func pickf64(ok bool, a, b float64) float64 {
	if ok {
		return a
	}
	return b
}

var speeds = []float64{0.5, 0.75, 1, 1.25, 1.5, 2}

func (p *player) nextSpeed() {
	i := 0
	for k, s := range speeds {
		if s == p.st.Rate {
			i = k
		}
	}
	r := speeds[(i+1)%len(speeds)]
	p.st.Rate = r
	p.send("rate", r)
	p.wake()
}

func (p *player) wake() { p.lastActive = time.Now() }

const idleAfter = 3 * time.Second

// idle reports whether the controls fade out: the video plays and nothing
// has moved for a while.
func (p *player) idle() bool {
	return !p.st.Paused && p.failure == "" && !p.preparing && time.Since(p.lastActive) > idleAfter
}

// ---- keyboard ----

func (p *player) keys(c *ui.Context) {
	handled := func(fn func()) { fn(); p.wake() }
	switch {
	case c.Shortcut(0, ui.KeyEscape):
		handled(func() {
			if p.full {
				p.setFull(false)
			} else {
				p.close()
			}
		})
	case c.Shortcut(0, ui.KeySpace), c.Shortcut(0, ui.KeyK):
		handled(p.toggle)
	case c.Shortcut(0, ui.KeyLeft), c.Shortcut(0, ui.KeyJ):
		handled(func() { p.skip(-10) })
	case c.Shortcut(0, ui.KeyRight), c.Shortcut(0, ui.KeyL):
		handled(func() { p.skip(10) })
	case c.Shortcut(0, ui.KeyUp):
		handled(func() { p.setVolume(p.st.Volume + 0.1) })
	case c.Shortcut(0, ui.KeyDown):
		handled(func() { p.setVolume(p.st.Volume - 0.1) })
	case c.Shortcut(0, ui.KeyM):
		handled(p.toggleMute)
	case c.Shortcut(0, ui.KeyF):
		handled(func() { p.setFull(!p.full) })
	case c.Shortcut(0, ui.KeyP):
		handled(func() { p.send("pip", 0) })
	case c.Shortcut(0, ui.KeyI):
		handled(func() {
			if p.full {
				p.setFull(false)
			}
			p.mini = true
		})
	}
}

// ---- views ----

func clock(sec float64) string {
	if math.IsNaN(sec) || math.IsInf(sec, 0) || sec < 0 {
		sec = 0
	}
	s := int(sec)
	h, m, s := s/3600, s%3600/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// scrim is the translucent black the controls sit on, as light as the video
// allows: white text on it reads on any frame.
var (
	onScrim   = ui.Hex("#ffffff")
	scrimDim  = ui.RGBA(0, 0, 0, 0.7)
	scrimSoft = ui.RGBA(0, 0, 0, 0)
)

func (p *player) fullView(c *ui.Context, pal *palette) {
	p.keys(c)
	root := ui.Box(c.Key("player")).Fill().Background(ui.Hex("#000000")).Focusable().AutoFocus().FocusRing(false)
	// the controls wake when the pointer moves
	if x, y, in := root.PointerPosition(); in {
		if pt := (ui.Point{X: x, Y: y}); pt != p.lastPtr {
			p.lastPtr = pt
			p.wake()
		}
	}
	idle := p.idle()
	if !p.st.Paused && p.failure == "" {
		c.After(300 * time.Millisecond) // to notice the idle time passing
	}
	if idle {
		root.Cursor(ui.CursorNone)
	}
	root.Children(func() {
		if p.web != nil {
			ui.WebView(c, p.web).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		}

		// the surface: click plays and pauses, double click goes full screen
		surface := ui.ButtonBase(c.Key("surface")).KeepFocus().Absolute().Top(0).Left(0).Right(0).Bottom(0).FocusRing(false).Label("プレイヤー")
		if surface.DoubleClicked() {
			p.toggle() // the first click of the pair paused it: undo
			p.setFull(!p.full)
		} else if surface.Clicked() {
			p.toggle()
		}

		p.pipNote(c, pal, false)
		fade := root.AnimateTo("controls", pickf(idle, 0, 1), spatialSlow)
		p.status(c, pal)
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Justify(ui.SpaceBetween).PassThrough().Opacity(fade).Children(func() {
			p.topBar(c, pal, fade > 0.05)
			p.bottomBar(c, pal, fade > 0.05)
		})
		if p.st.Paused && !p.preparing && p.failure == "" && !p.st.Buffering {
			p.centerPlay(c, pal)
		}
	})
}

// scrimBar lays a gradient behind the controls at an edge.
func scrimBar(e ui.Element, top bool) ui.Element {
	if top {
		return e.Gradient(scrimDim, scrimSoft, 180)
	}
	return e.Gradient(scrimSoft, scrimDim, 180)
}

func (p *player) topBar(c *ui.Context, pal *palette, live bool) {
	bar := ui.Row(c.Key("top")).KeepFocus().Height(72).Padding(0, 16).Gap(8).Center().Justify(ui.Start)
	bar.Gradient(scrimDim, scrimSoft, 180)
	bar.Children(func() {
		if live {
			if iconButton(c, pal, "close", "戻る", icBack, onScrim, false).Clicked() {
				p.close()
			}
		} else {
			ui.Box(c).Size(40, 40)
		}
		text(c, p.title, tsTitleLarge, onScrim).Grow(1).SingleLine()
	})
}

func (p *player) centerPlay(c *ui.Context, pal *palette) {
	b := ui.ButtonBase(c.Key("center")).KeepFocus().Absolute().Top(0).Left(0).Right(0).Bottom(0).Margin(ui.Auto).Size(80, 80).FocusRing(true)
	hov, prs := b.Hovered(), b.Pressed()
	r := b.AnimateTo("r", pickf(prs, 24, 40), spatialFast)
	s := b.AnimateTo("s", pickf(hov, 1.06, 1), spatial)
	b.Scale(s, s).Radius(r).Center().Background(layer(pal.primaryContainer, pal.onPrimaryContainer, hov, prs)).
		TextColor(pal.onPrimaryContainer).Label("再生").Cursor(ui.CursorPointer).Children(func() {
		ui.Icon(c, icPlay).FontSize(40)
	})
	if b.Clicked() {
		p.toggle()
	}
}

// status shows the preparation steps, the busy indicator or the error.
func (p *player) status(c *ui.Context, pal *palette) {
	showSteps := len(p.steps) > 0 && !p.st.Started && p.failure == ""
	switch {
	case p.failure != "":
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(16).Children(func() {
			ui.Column(c).Width(420).MaxWidth(420).Padding(24).Gap(16).AlignItems(ui.Center).Radius(shapeXL).Background(pal.surfaceContainerHigh).Children(func() {
				ui.Icon(c, icWarning).FontSize(40).TextColor(pal.primary)
				text(c, p.failure, tsBodyMedium, pal.onSurface).TextAlign(ui.Center).Selectable()
				for _, s := range p.steps {
					if s.state == 2 && s.detail != "" {
						text(c, s.label+": "+s.detail, tsLabelMedium, pal.onSurfaceVariant).FontWeight(400).TextAlign(ui.Center)
					}
				}
				if button(c, pal, "retry", "再試行", icRefresh, btnTonal).Clicked() {
					p.start()
				}
			})
		})
	case p.preparing || p.st.Buffering || showSteps:
		ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(24).PassThrough().Children(func() {
			if !p.st.Paused || p.preparing || showSteps {
				loadingIndicator(c, pal, 64)
			}
			if showSteps {
				p.stepList(c, pal)
			}
		})
	}
}

func (p *player) stepList(c *ui.Context, pal *palette) {
	ui.Column(c).MinWidth(280).Padding(16, 24).Gap(8).Radius(shapeL).Background(ui.RGBA(0, 0, 0, 0.55)).Children(func() {
		for i, s := range p.steps {
			ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Start).Children(func() {
				mark := ui.Box(c).Size(20, 20).Center().Shrink(0)
				mark.Children(func() {
					switch s.state {
					case 0:
						ui.Spinner(c).Size(14, 14).TextColor(pal.primaryContainer.Mix(onScrim, 0.2))
					case 1:
						ui.Icon(c, icCheck).FontSize(16).TextColor(pal.inversePrimary)
					default:
						ui.Icon(c, icClose).FontSize(16).TextColor(pal.error)
					}
				})
				ui.Column(c).Grow(1).Children(func() {
					text(c, s.label, tsBodyMedium, onScrim).Opacity(pickf(s.state == 2, 0.7, 1))
					if s.detail != "" {
						text(c, s.detail, tsLabelMedium, onScrim).FontWeight(400).Opacity(0.7).Margin(2, 0, 0)
					}
				})
			})
		}
	})
}

func (p *player) bottomBar(c *ui.Context, pal *palette, live bool) {
	bar := ui.Column(c.Key("bottom")).KeepFocus().Padding(24, 16, 8).Gap(0)
	bar.Gradient(scrimSoft, scrimDim, 180)
	bar.Children(func() {
		p.seekBar(c, pal)
		ui.Row(c).Gap(4).Center().Justify(ui.Start).Children(func() {
			st := p.st
			if iconButton(c, pal, "play", pickstr(st.Paused, "再生", "一時停止"), pickic(st.Paused, icPlay, icPause), onScrim, false).Clicked() {
				p.toggle()
			}
			p.skipButton(c, pal, "back10", "10秒戻る", icReplay10, -10)
			p.skipButton(c, pal, "fwd10", "10秒進む", icForward10, 10)
			p.volume(c, pal)
			text(c, clock(st.Time)+" / "+pickstr(st.Duration > 0, clock(st.Duration), "--:--"), tsLabelLarge, onScrim).
				FontFeatures("tnum").Margin(0, 8).SingleLine()
			ui.Spacer(c)
			sp := button(c, pal, "speed", fmt.Sprintf("%gx", st.Rate), nil, btnText, onScrim)
			if sp.Clicked() {
				p.nextSpeed()
			}
			if iconButton(c, pal, "mini", "ミニプレイヤー", icMini, onScrim, false).Clicked() {
				if p.full {
					p.setFull(false)
				}
				p.mini = true
			}
			if st.PipSupported {
				if iconButton(c, pal, "pip", "ピクチャーインピクチャー", icPip, onScrim, st.Pip).Clicked() {
					p.send("pip", 0)
				}
			}
			if iconButton(c, pal, "full", pickstr(p.full, "全画面を終了", "全画面"), pickic(p.full, icFullExit, icFullscreen), onScrim, false).Clicked() {
				p.setFull(!p.full)
			}
		})
	})
}

// skipButton is an icon button for the 10 second jumps, with the number over the arrow.
func (p *player) skipButton(c *ui.Context, pal *palette, key, label string, ic *ui.SVG, d float64) {
	b := ui.ButtonBase(c.Key(key))
	hov, prs := b.Hovered(), b.Pressed()
	bg := ui.Transparent
	if prs {
		bg = onScrim.Alpha(0.12)
	} else if hov {
		bg = onScrim.Alpha(0.08)
	}
	r := float32(20)
	ripple(b, onScrim, r, false)
	b.Size(40, 40).Center().Radius(r).Background(bg).TextColor(onScrim).Label(label).Tooltip(label).Cursor(ui.CursorPointer).Children(func() {
		ui.Icon(c, ic).FontSize(24)
		text(c, "10", typeStyle{8, 800, 10}, onScrim).Absolute().Top(15).Left(0).Right(0).TextAlign(ui.Center).PassThrough()
	})
	if b.Clicked() {
		p.skip(d)
	}
}

// volume is the mute button with a slider that opens beside it.
func (p *player) volume(c *ui.Context, pal *palette) {
	grp := ui.Row(c.Key("volume")).Center()
	open := grp.Hovered() || grp.FocusWithin()
	w := grp.AnimateTo("w", pickf(open, 112, 0), spatial)
	grp.Children(func() {
		level := pickf64(p.st.Muted, 0, p.st.Volume)
		if iconButton(c, pal, "mute", pickstr(p.st.Muted, "ミュート解除", "ミュート"), pickic(level == 0, icMuted, icVolume), onScrim, false).Clicked() {
			p.toggleMute()
		}
		if w > 1 {
			v := level
			s := ui.SliderBase(c.Key("vol"), &v, 0, 1).Width(w).Height(40).PaddingX(8).Opacity(w / 112).Label("音量")
			s.Step(0.05)
			s.Draw(func(pt *ui.Painter, r ui.Rect) {
				x0, x1 := r.X+8, r.X+r.W-8
				cy := r.Y + r.H/2
				pt.Fill(ui.Rect{X: x0, Y: cy - 2, W: x1 - x0, H: 4}, onScrim.Alpha(0.3), 2)
				at := x0 + (x1-x0)*float32(v)
				pt.Fill(ui.Rect{X: x0, Y: cy - 2, W: at - x0, H: 4}, onScrim, 2)
				pt.Fill(ui.Rect{X: at - 6, Y: cy - 6, W: 12, H: 12}, onScrim, 6)
			})
			if s.Changed() {
				p.setVolume(v)
			}
		}
	})
}

// seekBar is Expressive's slider with a wavy track: the played part waves
// while the video plays and straightens when it pauses.
func (p *player) seekBar(c *ui.Context, pal *palette) {
	dur := p.st.Duration
	v := 0.0
	if dur > 0 {
		v = math.Min(1, p.st.Time/dur)
	}
	s := ui.SliderBase(c.Key("seek"), &v, 0, 1).Height(28).PaddingX(8).Label("シーク").Cursor(ui.CursorPointer)
	amp := s.AnimateTo("amp", pickf(p.st.Paused || p.preparing, 0, 3), spatial)
	s.Draw(func(pt *ui.Painter, r ui.Rect) {
		x0, x1 := r.X+8, r.X+r.W-8
		cy := r.Y + r.H/2
		at := x0 + (x1-x0)*float32(v)
		// remaining track, then the played part over it
		pt.Fill(ui.Rect{X: at + 6, Y: cy - 2, W: max(0, x1-at-6), H: 4}, onScrim.Alpha(0.3), 2)
		phase := float32(0)
		if amp > 0.05 {
			phase = -float32(pt.Now().UnixMilli()%2000) / 2000 * 2 * math.Pi
			pt.AnimationFrame()
		}
		wavy(pt, x0, at, cy, amp, 24, phase, 4, pal.inversePrimary)
		pt.Fill(ui.Rect{X: at - 2, Y: cy - 11, W: 4, H: 22}, pal.inversePrimary, 2)
	})
	if s.Changed() && dur > 0 {
		p.seekTo(v * dur)
	}
}

// ---- mini player ----

func (p *player) miniView(c *ui.Context, pal *palette) {
	p.keys(c)
	box := ui.Box(c.Key("mini")).Absolute().Right(16).Bottom(16).Width(360).AspectRatio(16.0/9).Radius(shapeM).
		Background(ui.Hex("#000000")).Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.35))
	hov := box.Hovered() || box.FocusWithin()
	box.Children(func() {
		if p.web != nil {
			ui.WebView(c, p.web).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(shapeM)
		}
		surface := ui.ButtonBase(c.Key("surface")).KeepFocus().Absolute().Top(0).Left(0).Right(0).Bottom(0).FocusRing(false).Label("元のサイズに戻す")
		if surface.Clicked() {
			p.mini = false
		}
		p.pipNote(c, pal, true)
		if p.preparing || p.st.Buffering || (p.failure == "" && len(p.steps) > 0 && !p.st.Started) {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() { loadingIndicator(c, pal, 40) })
		}
		if p.failure != "" {
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(8).Padding(16).Background(pal.scrim.Alpha(0.6)).PassThrough().Children(func() {
				text(c, p.failure, tsLabelMedium, onScrim).TextAlign(ui.Center).MaxLines(3)
			})
		}
		op := box.AnimateTo("ctl", pickf(hov || p.st.Paused, 1, 0), spatial)
		ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Height(56).Padding(0, 8).Gap(4).Center().Justify(ui.Start).Opacity(op).
			Gradient(scrimSoft, scrimDim, 180).Children(func() {
			if iconButton(c, pal, "play", pickstr(p.st.Paused, "再生", "一時停止"), pickic(p.st.Paused, icPlay, icPause), onScrim, false).Clicked() {
				p.toggle()
			}
			ui.Spacer(c)
			if iconButton(c, pal, "expand", "元のサイズに戻す", icExpand, onScrim, false).Clicked() {
				p.mini = false
			}
			if iconButton(c, pal, "close", "閉じる", icClose, onScrim, false).Clicked() {
				p.close()
			}
		})
	})
}

// pipNote covers the page while its video plays in a picture-in-picture
// window. The page shows its own note then, in the web engine's language
// (English), so this paints over it with one in Japanese.
func (p *player) pipNote(c *ui.Context, pal *palette, small bool) {
	if !p.st.Pip {
		return
	}
	ui.Column(c.Key("pip")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Background(ui.Hex("#000000")).Children(func() {
		ui.Row(c).Gap(12).Padding(pickf(small, 8, 12), pickf(small, 16, 24)).Center().Radius(shapeM).Background(pal.surfaceContainerHigh).Children(func() {
			ui.Icon(c, icPip).FontSize(pickf(small, 18, 24)).TextColor(pal.onSurfaceVariant)
			text(c, "ピクチャーインピクチャーで再生中", pickts(small, tsLabelLarge, tsBodyLarge), pal.onSurface).SingleLine()
		})
	})
}

func pickts(ok bool, a, b typeStyle) typeStyle {
	if ok {
		return a
	}
	return b
}
