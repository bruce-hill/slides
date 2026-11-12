// slides - a terminal slide presenter
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/bruce-hill/slides/internal/deck"
	"github.com/bruce-hill/slides/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s file1.slides [file2.slides...]\n", os.Args[0])
		os.Exit(1)
	}

	var slides []deck.Slide
	for _, filename := range os.Args[1:] {
		s, err := deck.Load(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		slides = append(slides, s...)
	}

	if _, err := tea.NewProgram(ui.New(slides)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
