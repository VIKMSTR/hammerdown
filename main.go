// Command mkup is a Markdown editor with live preview built on Fyne.
//
// Usage: mkup [file.md]
package main

import (
	"os"

	"mkup/internal/ui"
)

func main() {
	ui.Run(os.Args[1:])
}
