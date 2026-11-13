// slides - a terminal slide presenter
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/bruce-hill/slides/internal/deck"
	"github.com/bruce-hill/slides/internal/ui"
)

func main() {
	start := flag.Int("n", 1, "start at slide `number`")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] file...\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Presents markdown files as slides. Other files are shown as a single code slide,\nand - reads markdown from standard input.\n\nOptions:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *version {
		v := "(unknown)"
		if info, ok := debug.ReadBuildInfo(); ok {
			v = info.Main.Version
		}
		fmt.Println("slides", v)
		return
	}
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var slides []deck.Slide
	for _, filename := range flag.Args() {
		s, err := deck.Load(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		slides = append(slides, s...)
	}

	if _, err := tea.NewProgram(ui.New(slides, *start-1)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
