package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo/ui"
)

// Google Sans Flex for Latin text and Noto Sans JP for Japanese, both under
// the SIL Open Font License (fonts/OFL-*.txt). They are variable fonts, so
// every weight of the type scale comes from one file.

//go:embed fonts/GoogleSansFlex.ttf
var googleSansFlex []byte

//go:embed fonts/NotoSansJP.ttf
var notoSansJP []byte

// fontFamily lists them in order: what the first lacks (Japanese) comes from the next.
const fontFamily = "Google Sans Flex, Noto Sans JP"

func init() {
	for _, f := range []struct {
		data []byte
		name string
	}{{googleSansFlex, "Google Sans Flex"}, {notoSansJP, "Noto Sans JP"}} {
		if err := ui.RegisterFont(f.data, f.name); err != nil {
			log.Printf("font %s: %v", f.name, err)
		}
	}
}
