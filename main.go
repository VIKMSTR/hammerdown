// Command hammerdown is a Markdown editor with live preview built on Fyne.
//
// Usage: hammerdown [file.md]
package main

import (
	"os"

	"hammerdown/internal/ui"
)

func main() {
	ui.Run(os.Args[1:])
}
