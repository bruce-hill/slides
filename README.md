# slides - A terminal slide presenter

Slides is a simple terminal application for presenting markdown files, built
with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Running

To run the slides program from this directory, run `go run ./cmd/slides sample.md`

Slides are separated by lines of three or more dashes (`---`). Any non-markdown
file passed on the command line is shown as a single syntax-highlighted slide.

## Features

- Markdown formatting, lists, block quotes, tables, and syntax-highlighted code
  blocks. Text is wrapped to fit the terminal
- ` ```demo ` code blocks are embedded terminals running bash, with the code
  typed in at the prompt, ready to run. Click a terminal (or press **Enter**
  when it's focused) to type in it, and press **Ctrl-]** to return to the
  slides. Your `~/.bashrc` is loaded, but the prompt is replaced with a simple
  `>`. Terminals keep running when you change slides. They fill the space left
  over by the rest of the slide, or ` ```demo height=10 ` sets their height
- Images (`![](image.png)`) are drawn in the terminal, sized to fill the space
  left over by the rest of the slide (up to 80% of the width, and at most twice
  their natural size). `![width=50%](...)` and `![height=50%](...)` set the
  maximum width or height as a percentage of the terminal instead. In terminals that support the
  [kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/)
  with Unicode placeholders (like kitty and Ghostty), images are shown at full
  resolution; other terminals get a lower resolution version made of colored
  characters
- Other files can be embedded with `![title](file.txt)`. Files taller than half
  the screen are shown in a box that scrolls with the mouse wheel
- Links can be opened with **Enter**

## Keys

| Key                              | Action                                 |
|----------------------------------|----------------------------------------|
| Right, Space, l                  | Next slide                             |
| Left, Backspace, h               | Previous slide                         |
| Home, g / End, G                 | First / last slide                     |
| 0-9 then Enter                   | Go to slide number                     |
| / then Enter, n, p               | Search, next/previous match            |
| Up/k, Down/j, mouse wheel        | Scroll                                 |
| Ctrl-U, Ctrl-D, PgUp, PgDn       | Scroll by 10 lines                     |
| Tab, Shift-Tab                   | Cycle the focused demo, link, or image |
| Enter                            | Type in the focused demo, or open the  |
|                                  | focused link or image                  |
| z                                | Type in the focused demo, zoomed to    |
|                                  | fill the screen                        |
| Ctrl-]                           | Stop typing in a demo                  |
| r, Ctrl-R                        | Redraw                                 |
| `                                | Toggle viewing the slide's source      |
| Ctrl-Z                           | Suspend                                |
| q, Ctrl-C                        | Quit                                   |

## Building

To build a standalone executable called `./slides`, run `make`. Run the tests
with `make test`.

## Installing

To install, run `make install` or `make PREFIX=/usr/local install` if you want
to install to a location other than `~/.local/bin`. You can also install with
`go install github.com/bruce-hill/slides/cmd/slides@latest`.

## Project Layout

```
cmd/slides/          Command line entry point
internal/deck/       Loading files and splitting them into slides
internal/render/     Rendering markdown (and images) as terminal text
internal/term/       Shells running in virtual terminals, for demos
internal/highlight/  Syntax highlighting, with Tomo, Nomsu and BP lexers (lexers/*.xml)
internal/ui/         The Bubble Tea presenter (keys, scrolling, status bar)
```
