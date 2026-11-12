//go:build unix

package ui

import (
	"os"

	"golang.org/x/sys/unix"
)

// cellSize returns the size of a character cell in pixels, if known.
func cellSize() (w, h int, ok bool) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return 0, 0, false
	}
	return int(ws.Xpixel / ws.Col), int(ws.Ypixel / ws.Row), true
}
