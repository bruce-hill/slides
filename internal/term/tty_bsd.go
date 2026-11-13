//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package term

import "golang.org/x/sys/unix"

const ioctlGetTermios = unix.TIOCGETA
