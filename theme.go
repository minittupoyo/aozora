package main

import "github.com/egoist/mygo/ui"

// palette is a Material 3 color scheme. The values are the ones
// material-color-utilities generates from the seed #0061a4 (tonal spot), which
// the web version of the app used through mdui.
type palette struct {
	dark bool

	primary, onPrimary, primaryContainer, onPrimaryContainer         ui.Color
	secondary, onSecondary, secondaryContainer, onSecondaryContainer ui.Color
	tertiary, onTertiary, tertiaryContainer, onTertiaryContainer     ui.Color
	error, onError, errorContainer, onErrorContainer                 ui.Color
	surface, onSurface, surfaceVariant, onSurfaceVariant             ui.Color
	outline, outlineVariant                                          ui.Color
	inverseSurface, inverseOnSurface, inversePrimary                 ui.Color
	surfaceContainerLowest, surfaceContainerLow, surfaceContainer    ui.Color
	surfaceContainerHigh, surfaceContainerHighest                    ui.Color
	scrim                                                            ui.Color
}

var lightPalette = palette{
	primary: ui.Hex("#36618e"), onPrimary: ui.Hex("#ffffff"),
	primaryContainer: ui.Hex("#d1e4ff"), onPrimaryContainer: ui.Hex("#1a4975"),
	secondary: ui.Hex("#535f70"), onSecondary: ui.Hex("#ffffff"),
	secondaryContainer: ui.Hex("#d7e3f8"), onSecondaryContainer: ui.Hex("#3b4858"),
	tertiary: ui.Hex("#6b5778"), onTertiary: ui.Hex("#ffffff"),
	tertiaryContainer: ui.Hex("#f3daff"), onTertiaryContainer: ui.Hex("#523f5f"),
	error: ui.Hex("#ba1a1a"), onError: ui.Hex("#ffffff"),
	errorContainer: ui.Hex("#ffdad6"), onErrorContainer: ui.Hex("#93000a"),
	surface: ui.Hex("#f8f9ff"), onSurface: ui.Hex("#191c20"),
	surfaceVariant: ui.Hex("#dfe2eb"), onSurfaceVariant: ui.Hex("#43474e"),
	outline: ui.Hex("#73777f"), outlineVariant: ui.Hex("#c3c6cf"),
	inverseSurface: ui.Hex("#2e3135"), inverseOnSurface: ui.Hex("#eff0f7"), inversePrimary: ui.Hex("#a0cafd"),
	surfaceContainerLowest: ui.Hex("#ffffff"), surfaceContainerLow: ui.Hex("#f2f3fa"),
	surfaceContainer: ui.Hex("#eceef4"), surfaceContainerHigh: ui.Hex("#e6e8ee"),
	surfaceContainerHighest: ui.Hex("#e1e2e8"),
	scrim:                   ui.Hex("#000000"),
}

var darkPalette = palette{
	dark:    true,
	primary: ui.Hex("#a0cafd"), onPrimary: ui.Hex("#003258"),
	primaryContainer: ui.Hex("#1a4975"), onPrimaryContainer: ui.Hex("#d1e4ff"),
	secondary: ui.Hex("#bbc7db"), onSecondary: ui.Hex("#253140"),
	secondaryContainer: ui.Hex("#3b4858"), onSecondaryContainer: ui.Hex("#d7e3f8"),
	tertiary: ui.Hex("#d7bee4"), onTertiary: ui.Hex("#3b2948"),
	tertiaryContainer: ui.Hex("#523f5f"), onTertiaryContainer: ui.Hex("#f3daff"),
	error: ui.Hex("#ffb4ab"), onError: ui.Hex("#690005"),
	errorContainer: ui.Hex("#93000a"), onErrorContainer: ui.Hex("#ffdad6"),
	surface: ui.Hex("#111418"), onSurface: ui.Hex("#e1e2e8"),
	surfaceVariant: ui.Hex("#43474e"), onSurfaceVariant: ui.Hex("#c3c6cf"),
	outline: ui.Hex("#8d9199"), outlineVariant: ui.Hex("#43474e"),
	inverseSurface: ui.Hex("#e1e2e8"), inverseOnSurface: ui.Hex("#2e3135"), inversePrimary: ui.Hex("#36618e"),
	surfaceContainerLowest: ui.Hex("#0b0e13"), surfaceContainerLow: ui.Hex("#191c20"),
	surfaceContainer: ui.Hex("#1d2024"), surfaceContainerHigh: ui.Hex("#272a2f"),
	surfaceContainerHighest: ui.Hex("#32353a"),
	scrim:                   ui.Hex("#000000"),
}

// Shape scale. Material 3 Expressive adds the large-increased and
// extra-large-increased steps and leans on bigger, rounder corners.
const (
	shapeXS    = 4
	shapeS     = 8
	shapeM     = 12
	shapeL     = 16
	shapeLInc  = 20
	shapeXL    = 28
	shapeXLInc = 32
	shapeFull  = 999
)

// Motion: Expressive replaces easing curves with springs.
var (
	spatialFast = ui.Animation{Spring: ui.SpringOptions{Stiffness: 800, Damping: 45, Mass: 1}}
	spatial     = ui.Animation{Spring: ui.SpringOptions{Stiffness: 380, Damping: 31, Mass: 1}}
	spatialSlow = ui.Animation{Spring: ui.SpringOptions{Stiffness: 200, Damping: 24, Mass: 1}}
	effects     = ui.Animation{Duration: 150e6, Ease: ui.EaseOut}
)

// pal returns the color scheme for the window's current appearance and makes
// the toolkit's own theme (text, scroll bars, toasts, focus rings) match it.
// Call it once at the top of the view.
func pal(c *ui.Context) *palette {
	base := *c.Theme()
	p := &lightPalette
	if base.Dark {
		p = &darkPalette
	}
	t := base
	t.Background = p.surface
	t.Surface = p.surfaceContainerHigh
	t.SurfaceHover = p.surfaceContainerHighest
	t.SurfacePressed = p.surfaceContainerHighest
	t.Border = p.outlineVariant
	t.Text = p.onSurface
	t.TextMuted = p.onSurfaceVariant
	t.Accent = p.primary
	t.AccentHover = p.primary
	t.AccentPressed = p.primary
	t.AccentText = p.onPrimary
	t.Danger = p.error
	t.Focus = p.primary
	t.Inverse = p.inverseSurface
	t.InverseText = p.inverseOnSurface
	t.Scrollbar = p.outline.Alpha(0.6)
	t.ScrollbarWidth = 8
	t.Selection = p.primary.Alpha(0.3)
	t.Radius = shapeM
	t.Font = fontFamily
	c.SetTheme(&t)
	return p
}

// Type scale (size, weight). Expressive emphasizes with heavier weights.
type typeStyle struct {
	size   float32
	weight int
	line   float32
}

var (
	tsHeadlineMedium = typeStyle{28, 600, 36}
	tsHeadlineSmall  = typeStyle{24, 600, 32}
	tsTitleLarge     = typeStyle{22, 500, 28}
	tsTitleMedium    = typeStyle{16, 600, 24}
	tsTitleSmall     = typeStyle{14, 600, 20}
	tsBodyLarge      = typeStyle{16, 400, 24}
	tsBodyMedium     = typeStyle{14, 400, 20}
	tsBodySmall      = typeStyle{12, 400, 16}
	tsLabelLarge     = typeStyle{14, 600, 20}
	tsLabelMedium    = typeStyle{12, 600, 16}
	tsLabelSmall     = typeStyle{11, 600, 16}
)

// text builds a Text in a type style and color.
func text(c *ui.Context, s string, st typeStyle, col ui.Color) ui.Element {
	return ui.Text(c, s).FontSize(st.size).FontWeight(st.weight).FixedLineHeight(st.line).TextColor(col)
}
