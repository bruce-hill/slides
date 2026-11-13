//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package term

import (
	"os"

	"golang.org/x/sys/unix"
)

// lineEditing reports whether the program in a terminal is reading input with
// a line editor (like readline), which turns off the terminal's line mode.
func lineEditing(pty *os.File) bool {
	conn, err := pty.SyscallConn()
	if err != nil {
		return true
	}
	editing := true
	conn.Control(func(fd uintptr) {
		if t, err := unix.IoctlGetTermios(int(fd), ioctlGetTermios); err == nil {
			editing = t.Lflag&unix.ICANON == 0
		}
	})
	return editing
}
