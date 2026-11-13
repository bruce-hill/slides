//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd)

package term

import "os"

func lineEditing(pty *os.File) bool { return true }
