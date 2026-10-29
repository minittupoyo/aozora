package main

import (
	"context"

	"github.com/egoist/mygo"
)

// SetFullScreen puts the window that called it into or out of OS-level full
// screen (covering the taskbar), and reports whether it is now full screen.
func (t *Tver) SetFullScreen(ctx context.Context, on bool) bool {
	win := mygo.CallerWindow(ctx)
	if win == nil {
		return false
	}
	win.SetFullScreen(on)
	return on
}
