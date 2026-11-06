package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.Bind(&PlayerBridge{})

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "Aozora",
			Width:           1200,
			Height:          780,
			MinWidth:        720,
			MinHeight:       480,
			BackgroundColor: "light-dark(#f8f9ff, #111418)", // the surface color of the palette
			StateKey:        "main",
			Content:         ui.View(func(c *ui.Context) { theApp.view(c) }),
		})
		theApp = newApp(win)
		theApp.showHome()
		win.Invalidate()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
