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
  blocks
- ` ```run ` code blocks are run in bash and show their output
- ` ```demo ` code blocks are run interactively when you press **Enter**
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
| Right, Space, j                  | Next slide                             |
| Left, Backspace, k               | Previous slide                         |
| Home, h / End, l                 | First / last slide                     |
| 0-9 then Enter                   | Go to slide number                     |
| / then Enter, n, p               | Search, next/previous match            |
| Up, Down, mouse wheel            | Scroll                                 |
| Ctrl-U, Ctrl-D, PgUp, PgDn       | Scroll by 10 lines                     |
| Tab, Shift-Tab                   | Cycle the focused demo, link, or image |
| Enter                            | Run/open the focused item              |
| r, Ctrl-R                        | Redraw and rerun `run` blocks          |
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
internal/highlight/  Syntax highlighting, with Tomo, Nomsu and BP lexers (lexers/*.xml)
internal/ui/         The Bubble Tea presenter (keys, scrolling, status bar)
```
