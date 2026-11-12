//go:build !unix

package ui

func cellSize() (w, h int, ok bool) { return 0, 0, false }
