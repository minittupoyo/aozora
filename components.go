package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// ---- state layers ----

// layer mixes the state layer of an interaction into a container color, as
// Material 3 does: 8% on hover, 10% on press and focus.
func layer(base, content ui.Color, hovered, pressed bool) ui.Color {
	if hovered || pressed {
		return base.Mix(content, 0.08)
	}
	return base
}

// ---- buttons ----

type btnKind int

const (
	btnFilled btnKind = iota
	btnTonal
	btnText
	btnOutlined
)

// button is a Material 3 button with an optional leading icon. Like the
// toolkit's own, it reports clicks through the element it returns. On press
// its corners square off, the shape change Expressive uses as feedback.
func button(c *ui.Context, p *palette, key, label string, ic *ui.SVG, kind btnKind, tint ...ui.Color) ui.Element {
	var bg, fg ui.Color
	switch kind {
	case btnFilled:
		bg, fg = p.primary, p.onPrimary
	case btnTonal:
		bg, fg = p.secondaryContainer, p.onSecondaryContainer
	case btnText:
		bg, fg = ui.Transparent, p.primary
	case btnOutlined:
		bg, fg = ui.Transparent, p.primary
	}
	if len(tint) > 0 {
		fg = tint[0]
	}
	b := ui.ButtonBase(c.Key(key))
	hov, prs := b.Hovered(), b.Pressed()
	if b.IsDisabled() {
		hov, prs = false, false
	}
	r := b.AnimateTo("r", pickf(prs, 12, 20), spatialFast)
	cur := layer(bg, fg, hov, prs)
	if kind == btnText || kind == btnOutlined {
		cur = ui.Transparent
		if prs {
			cur = fg.Alpha(0.10)
		} else if hov {
			cur = fg.Alpha(0.08)
		}
	}
	ripple(b, fg, r, false)
	b.Height(40).Padding(0, pickf(ic != nil, 16, 24)).Gap(8).Center().Radius(r).Background(cur).TextColor(fg)
	if kind == btnOutlined {
		b.Border(1, p.outline)
	}
	if b.IsDisabled() {
		b.Opacity(0.38)
	}
	b.Cursor(ui.CursorPointer).Label(label).Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(18)
		}
		text(c, label, tsLabelLarge, fg).SingleLine()
	})
	return b
}

// iconButton is a 40 dp round icon button. tint overrides the icon color; on
// marks a toggle that is on, which is drawn tonal.
func iconButton(c *ui.Context, p *palette, key, label string, ic *ui.SVG, tint ui.Color, on bool) ui.Element {
	b := ui.ButtonBase(c.Key(key))
	hov, prs := b.Hovered(), b.Pressed()
	fg := tint
	if fg == (ui.Color{}) {
		fg = p.onSurfaceVariant
	}
	bg := ui.Transparent
	if on {
		bg, fg = p.secondaryContainer, p.onSecondaryContainer
	}
	if prs {
		bg = bg.Mix(fg, 0.12)
		if bg.A == 0 {
			bg = fg.Alpha(0.12)
		}
	} else if hov {
		if bg.A == 0 {
			bg = fg.Alpha(0.08)
		} else {
			bg = bg.Mix(fg, 0.08)
		}
	}
	r := b.AnimateTo("r", pickf(prs, 12, 20), spatialFast)
	ripple(b, fg, r, false)
	b.Size(40, 40).Center().Radius(r).Background(bg).TextColor(fg).Cursor(ui.CursorPointer).
		Label(label).Tooltip(label).Children(func() { ui.Icon(c, ic).FontSize(24) })
	return b
}

func pickf(ok bool, a, b float32) float32 {
	if ok {
		return a
	}
	return b
}

// ---- chips ----

// filterChip is a Material 3 filter chip: outlined, and tonal with a check
// when selected.
func filterChip(c *ui.Context, p *palette, key any, label string, selected bool) ui.Element {
	b := ui.ButtonBase(c.Key(key))
	hov, prs := b.Hovered(), b.Pressed()
	bg, fg := ui.Transparent, p.onSurfaceVariant
	if selected {
		bg, fg = p.secondaryContainer, p.onSecondaryContainer
	}
	if prs {
		bg = bg.Mix(fg, 0.10)
		if bg.A == 0 {
			bg = fg.Alpha(0.10)
		}
	} else if hov {
		if bg.A == 0 {
			bg = fg.Alpha(0.08)
		} else {
			bg = bg.Mix(fg, 0.08)
		}
	}
	ripple(b, fg, shapeS, false)
	b.Height(32).Padding(0, 16).PaddingStart(pickf(selected, 8, 16)).Gap(8).Center().Radius(shapeS).Background(bg).TextColor(fg).
		Cursor(ui.CursorPointer)
	if !selected {
		b.Border(1, p.outline)
	}
	b.Label(label).Children(func() {
		if selected {
			ui.Icon(c, icCheck).FontSize(18)
		}
		text(c, label, tsLabelLarge, fg).SingleLine()
	})
	return b
}

// connectedGroup is Expressive's connected button group: single-choice
// segments whose selected one is fully round and whose neighbors are
// squarer. It returns the index clicked, or -1.
func connectedGroup(c *ui.Context, p *palette, key string, labels []string, selected int) int {
	clicked := -1
	ui.Row(c.Key(key)).Gap(2).Children(func() {
		for i, l := range labels {
			b := ui.ButtonBase(c.Key(i))
			hov, prs := b.Hovered(), b.Pressed()
			bg, fg := p.surfaceContainerHigh, p.onSurface
			if i == selected {
				bg, fg = p.primary, p.onPrimary
			}
			bg = layer(bg, fg, hov, prs)
			r := b.AnimateTo("r", pickf(i == selected, 20, pickf(prs, 8, 12)), spatialFast)
			ripple(b, fg, r, false)
			b.Height(40).Padding(0, 20).Center().Radius(r).Background(bg).TextColor(fg).Cursor(ui.CursorPointer).
				Label(l).Children(func() {
				text(c, l, tsLabelLarge, fg).SingleLine()
			})
			if b.Clicked() {
				clicked = i
			}
		}
	})
	return clicked
}

// ---- progress ----

// wavy strokes a sine wave from x0 to x1, Expressive's wavy progress shape.
func wavy(p *ui.Painter, x0, x1, cy, amp, length, phase, width float32, col ui.Color) {
	if x1 <= x0 {
		return
	}
	pts := make([][2]float32, 0, int((x1-x0)/2)+2)
	for x := x0; ; x += 2 {
		if x > x1 {
			x = x1
		}
		y := cy + amp*float32(math.Sin(float64((x/length)*2*math.Pi+phase)))
		pts = append(pts, [2]float32{x, y})
		if x >= x1 {
			break
		}
	}
	p.StrokePolyline(pts, width, col)
}

// loadingBar is an indeterminate wavy progress bar along the top of the page.
func loadingBar(c *ui.Context, p *palette) {
	ui.Box(c).Height(8).Draw(func(pt *ui.Painter, r ui.Rect) {
		cy := r.Y + r.H/2
		pt.Fill(ui.Rect{X: r.X, Y: cy - 2, W: r.W, H: 4}, p.secondaryContainer, 2)
		t := float64(pt.Now().UnixMilli()%1800) / 1800
		// the head runs ahead of the tail, as the indeterminate bar of Material does
		head := ease(math.Min(1, t*1.6))
		tail := ease(math.Max(0, t*1.6-0.6))
		x0 := r.X + float32(tail)*r.W
		x1 := r.X + float32(head)*r.W
		wavy(pt, x0, x1, cy, 2, 28, -float32(t)*40, 4, p.primary)
		pt.AnimationFrame()
	})
}

func ease(t float64) float64 { return 1 - math.Pow(1-t, 3) }

// loadingIndicator is Expressive's loading indicator in spirit: a shape that
// turns while its corners swell and shrink.
func loadingIndicator(c *ui.Context, p *palette, size float32) {
	box := ui.Box(c).Size(size, size).Center()
	box.Background(p.primaryContainer).Radius(size / 2)
	box.Children(func() {
		inner := ui.Box(c).Size(size*0.5, size*0.5)
		t := inner.Loop("turn", 1400*time.Millisecond, ui.Linear)
		round := float32(0.5 - 0.5*math.Cos(float64(t)*4*math.Pi)) // 0 → 1 → 0, twice a turn
		inner.Radius(size * 0.5 * (0.22 + 0.78*round)).Background(p.onPrimaryContainer).Rotate(t * 360)
	})
}

// ---- states ----

// emptyState is an icon over a message, for pages without content.
func emptyState(c *ui.Context, p *palette, msg string) {
	ui.Column(c).AlignItems(ui.Center).Gap(8).Margin(96, 16).Children(func() {
		ui.Icon(c, icTv).FontSize(48).TextColor(p.outline)
		text(c, msg, tsBodyLarge, p.onSurfaceVariant).TextAlign(ui.Center).Selectable()
	})
}

// ---- ripple ----

type rippleState struct {
	x, y   float32
	t0, up time.Time
	down   bool
	active bool
}

const (
	rippleGrow = 400 * time.Millisecond
	rippleFade = 250 * time.Millisecond
)

// ripple gives an element Material 3's pressed state: a circle in the content
// color grows from where the pointer went down, to 10% opacity, and fades
// once the pointer is released. radius is the element's corner radius, which
// the ripple is clipped to. Call it before Children. With over true the
// painting is returned for rippleOver instead, for elements whose children
// (media) cover their background.
func ripple(b ui.Element, col ui.Color, radius float32, over bool) func(*ui.Painter, ui.Rect) {
	return rippleIn(b, col, radius, over, nil)
}

// rippleIn is ripple confined to a part of the element, which area gives from
// the element's box (the indicator of a navigation item). The circle still
// starts where the pointer went down.
func rippleIn(b ui.Element, col ui.Color, radius float32, over bool, area func(ui.Rect) ui.Rect) func(*ui.Painter, ui.Rect) {
	st := ui.Local(b, "ripple", func() rippleState { return rippleState{} })
	prs := b.Pressed()
	now := time.Now()
	if prs && !st.down {
		st.x, st.y, _ = b.PointerPosition()
		st.t0, st.down, st.active = now, true, true
	} else if !prs && st.down {
		st.down, st.up = false, now
	}
	paint := func(p *ui.Painter, box ui.Rect) {
		r := box
		if area != nil {
			r = area(box)
		}
		if !st.active {
			return
		}
		now := p.Now()
		grow := float64(now.Sub(st.t0)) / float64(rippleGrow)
		grow = ease(math.Min(1, grow))
		alpha := float32(0.10)
		if !st.down {
			// fade out after the circle has grown and the pointer is up
			start := st.up
			if end := st.t0.Add(rippleGrow); end.After(start) {
				start = end
			}
			f := float64(now.Sub(start)) / float64(rippleFade)
			if f >= 1 {
				st.active = false
				return
			}
			if f > 0 {
				alpha *= float32(1 - f)
			}
		}
		// the circle reaches the farthest corner
		cx, cy := box.X+st.x, box.Y+st.y
		dx := math.Max(float64(cx-r.X), float64(r.X+r.W-cx))
		dy := math.Max(float64(cy-r.Y), float64(r.Y+r.H-cy))
		rad := float32(math.Hypot(dx, dy) * grow)
		p.Clip(r, radius, func() {
			p.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, col.Alpha(alpha), rad)
		})
		p.AnimationFrame()
	}
	if over {
		return paint
	}
	b.Draw(paint)
	return nil
}

// rippleOver draws a ripple in the last child of an element whose children
// (a photo) would otherwise paint over it.
func rippleOver(c *ui.Context, paint func(*ui.Painter, ui.Rect)) {
	ui.Box(c.Key("ripple")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(paint)
}
