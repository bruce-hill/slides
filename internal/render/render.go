// Package render renders markdown slides as styled terminal text.
package render

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/bruce-hill/slides/internal/highlight"
)

var markdown = goldmark.New(goldmark.WithExtensions(extension.Strikethrough))

func Parse(src []byte) ast.Node {
	return markdown.Parser().Parse(text.NewReader(src))
}

var (
	green  = lipgloss.Color("2")
	yellow = lipgloss.Color("3")
	blue   = lipgloss.Color("4")
	cyan   = lipgloss.Color("6")
)

// wrap surrounds each line of s with the given SGR on/off codes, so that every
// line can be displayed on its own without losing its styling.
func wrap(on, off, s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "\x1b[" + on + "m" + line + "\x1b[" + off + "m"
	}
	return strings.Join(lines, "\n")
}

// MaxWidth returns the display width of the widest line of s.
func MaxWidth(s string) int {
	w := 0
	for _, line := range strings.Split(s, "\n") {
		w = max(w, ansi.StringWidth(line))
	}
	return w
}

type Options struct {
	Src          []byte
	Dir          string // Directory that relative paths are resolved from
	TermW, TermH int
	Focused      ast.Node               // The focused demo, link, or image
	RunCache     map[ast.Node]string    // Output of ```run blocks
	Kitty        *Kitty                 // If set, images use the kitty graphics protocol
	Scrolls      map[ast.Node]int       // Scroll position of each embedded file viewport
	FileCache    map[string]string      // Highlighted contents of embedded files
	ImageCache   map[string]image.Image // Decoded images
}

// Viewport is an embedded file that is too tall to show all at once, and is
// shown in a scrollable box instead.
type Viewport struct {
	Node ast.Node
	// Position and size of the box (including its border) in the rendered text
	Row, Col, Width, Height int
	Scroll                  int // First line shown
	MaxScroll               int
}

type renderer struct {
	Options
	width int // Width that headings and boxes expand to fill

	measuring bool // Whether images are being left out to measure the other content
	numImages int  // Number of images found while measuring
	imageRows int  // Maximum height of each image
	viewports []Viewport
}

func (r *renderer) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.Dir, path)
}

func (r *renderer) heading(title string, color color.Color) string {
	style := lipgloss.NewStyle().Bold(true).Reverse(true).Align(lipgloss.Center).
		Width(max(r.width, ansi.StringWidth(title)))
	if color != nil {
		style = style.Foreground(color)
	}
	return style.Render(title)
}

// box draws a border around text, optionally with line numbers.
func (r *renderer) box(text string, lineNumbers bool, color color.Color) string {
	lines := strings.Split(text, "\n")
	return r.boxLines(lines, lineNumbers, 1, len(lines), color)
}

// boxLines draws a border around lines of text. Line numbers start from first
// and are padded to fit the number total.
func (r *renderer) boxLines(lines []string, lineNumbers bool, first, total int, color color.Color) string {
	lines = append([]string(nil), lines...)
	numW := len(strconv.Itoa(total))
	bar := lipgloss.NewStyle().Foreground(color).Render("│")
	for i, line := range lines {
		if lineNumbers {
			lines[i] = fmt.Sprintf("\x1b[2m%*d\x1b[22m%s %s", numW, first+i, bar, line)
		} else {
			lines[i] = " " + line
		}
	}
	inner := max(r.width-2, MaxWidth(strings.Join(lines, "\n"))+1)
	for i, line := range lines {
		lines[i] = line + strings.Repeat(" ", inner-ansi.StringWidth(line))
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(color).
		Render(strings.Join(lines, "\n"))
}

func (r *renderer) blocks(n ast.Node, sep string) string {
	var parts []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if s := r.block(c); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep)
}

// CodeText returns the raw text of a code block.
func CodeText(n ast.Node, src []byte) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(src))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (r *renderer) block(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Heading:
		title := r.heading(" "+r.inlines(n)+" ", nil)
		if n.Level == 1 {
			bar := r.heading("", nil)
			return bar + "\n" + title + "\n" + bar
		}
		return title
	case *ast.Paragraph, *ast.TextBlock:
		out := strings.TrimSpace(r.inlines(n))
		// Center images that are on their own, not counting the focus marker after them
		if img, ok := n.FirstChild().(*ast.Image); ok && img == n.LastChild() && isImage(string(img.Destination)) {
			indent := strings.Repeat(" ", max(0, (r.width-(MaxWidth(out)-1))/2))
			out = indent + strings.ReplaceAll(out, "\n", "\n"+indent)
		}
		return out
	case *ast.List:
		sep := "\n\n"
		if n.IsTight {
			sep = "\n"
		}
		var items []string
		num := n.Start
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			bullet := "\x1b[1m  ◆ \x1b[22m"
			if n.IsOrdered() {
				bullet = fmt.Sprintf("\x1b[1m%2d.\x1b[22m ", num)
				num++
			}
			items = append(items, bullet+strings.ReplaceAll(r.blocks(c, sep), "\n", "\n    "))
		}
		return strings.Join(items, sep)
	case *ast.FencedCodeBlock:
		code := CodeText(n, r.Src)
		switch lang := string(n.Language(r.Src)); lang {
		case "run":
			prompt := "\x1b[33;1m$\x1b[m " + highlight.Highlight(code, highlight.LexerFor("bash"))
			return r.box(prompt+"\n"+r.run(n, code), false, yellow)
		case "demo":
			title := "Demo"
			if n == r.Focused {
				title = "Demo (press Enter to run)"
			}
			prompt := "\x1b[33;1m$\x1b[m " + highlight.Highlight(code, highlight.LexerFor("bash"))
			return r.heading(title, green) + "\n" + r.box(prompt, false, green)
		default:
			return r.box(highlight.Highlight(code, highlight.LexerFor(lang)), true, blue)
		}
	case *ast.CodeBlock:
		return r.box(highlight.Highlight(CodeText(n, r.Src), nil), true, blue)
	case *ast.Blockquote:
		return wrap("34;3", "39;23", r.blocks(n, "\n\n"))
	case *ast.ThematicBreak:
		return wrap("2", "22", strings.Repeat("─", r.width))
	case *ast.HTMLBlock:
		return CodeText(n, r.Src)
	default:
		return r.blocks(n, "\n\n")
	}
}

func (r *renderer) inlines(n ast.Node) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		sb.WriteString(r.inline(c))
	}
	return sb.String()
}

func (r *renderer) inline(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Text:
		s := string(n.Value(r.Src))
		if n.SoftLineBreak() || n.HardLineBreak() {
			s += "\n"
		}
		return s
	case *ast.String:
		return string(n.Value)
	case *ast.CodeSpan:
		return wrap("1;32;48;2;40;50;40", "22;39;49", r.inlines(n))
	case *ast.Emphasis:
		if n.Level >= 2 {
			return wrap("1", "22", r.inlines(n))
		}
		return wrap("3", "23", r.inlines(n))
	case *east.Strikethrough:
		return wrap("9", "29", r.inlines(n))
	case *ast.Link:
		return r.link(string(n.Destination), r.inlines(n), n)
	case *ast.AutoLink:
		return r.link(string(n.URL(r.Src)), "", n)
	case *ast.Image:
		return r.image(n)
	case *ast.RawHTML:
		var sb strings.Builder
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			sb.Write(seg.Value(r.Src))
		}
		return sb.String()
	default:
		return r.inlines(n)
	}
}

func (r *renderer) link(url, title string, n ast.Node) string {
	if title == "" {
		title = url
	}
	style := "4;34"
	if n == r.Focused {
		style = "1;4;34"
	}
	return ansi.SetHyperlink(url) + wrap(style, "22;24;39", title) + ansi.ResetHyperlink()
}

func plainText(n ast.Node, src []byte) string {
	var sb strings.Builder
	ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			sb.Write(t.Value(src))
		}
		return ast.WalkContinue, nil
	})
	return sb.String()
}

var (
	widthOptRe  = regexp.MustCompile(`width=(\d+)%`)
	heightOptRe = regexp.MustCompile(`height=(\d+)%`)
)

// imageSize returns the size in cells to display a w x h pixel image. Images
// fill the space left over by the rest of the slide, but aren't enlarged past
// twice their natural size, which would look blurry. The width=N% and
// height=N% options in the alt text replace the maximum width and height.
func (r *renderer) imageSize(alt string, w, h, cellW, cellH int) (cols, rows int) {
	maxCols, maxRows := float64(r.TermW*8/10), float64(r.imageRows)
	if m := widthOptRe.FindStringSubmatch(alt); m != nil {
		pct, _ := strconv.Atoi(m[1])
		maxCols = float64(r.TermW * pct / 100)
	}
	if m := heightOptRe.FindStringSubmatch(alt); m != nil {
		pct, _ := strconv.Atoi(m[1])
		maxRows = float64(r.TermH * pct / 100)
	}
	natCols, natRows := float64(w)/float64(cellW), float64(h)/float64(cellH)
	scale := min(maxCols/natCols, maxRows/natRows, 2)
	return max(1, int(natCols*scale)), max(1, int(natRows*scale))
}

func (r *renderer) image(n *ast.Image) string {
	path := r.resolve(string(n.Destination))
	alt := plainText(n, r.Src)
	if isImage(path) {
		if r.measuring {
			r.numImages++
			return " "
		}
		img, ok := r.ImageCache[path]
		if !ok {
			var err error
			if img, err = loadImage(path); err != nil {
				return wrap("31;1", "22;39", fmt.Sprintf("<Couldn't load image %s: %v>", n.Destination, err))
			}
			r.ImageCache[path] = img
		}
		// Half-block characters show two square pixels per cell
		cellW, cellH := 1, 2
		if r.Kitty != nil {
			cellW, cellH = r.Kitty.CellW, r.Kitty.CellH
		}
		cols, rows := r.imageSize(alt, img.Bounds().Dx(), img.Bounds().Dy(), cellW, cellH)

		var out string
		if r.Kitty != nil {
			out = r.Kitty.Placeholder(path, img, cols, rows)
		} else {
			out = imageToText(img, cols, rows)
		}
		marker := " "
		if n == r.Focused {
			marker = "\x1b[33m*\x1b[m"
		}
		if first, rest, ok := strings.Cut(out, "\n"); ok {
			return first + marker + "\n" + rest
		}
		return out + marker
	}

	// Embedded text file
	code, ok := r.FileCache[path]
	if !ok {
		contents, err := os.ReadFile(path)
		if err != nil {
			return wrap("31;1", "22;39", fmt.Sprintf("<File not found: %s>", n.Destination))
		}
		code = highlight.Highlight(string(contents), highlight.LexerFor(strings.TrimPrefix(filepath.Ext(path), ".")))
		if r.FileCache != nil {
			r.FileCache[path] = code
		}
	}
	title := r.inlines(n)
	if title == "" {
		title = path
	}
	return r.heading(title, cyan) + "\n" + r.viewport(n, code, cyan)
}

// Marks the top left corner of a viewport in the rendered text. It's removed
// once the viewport's position has been found.
var viewportMarkerRe = regexp.MustCompile("\x1b_viewport(\\d+)\x1b\\\\")

// viewport draws code in a box with line numbers, which scrolls if the code is
// too tall to fit on the slide.
func (r *renderer) viewport(n ast.Node, code string, color color.Color) string {
	lines := strings.Split(code, "\n")
	height := max(8, r.TermH/2)
	if len(lines) <= height {
		return r.box(code, true, color)
	}
	maxScroll := len(lines) - height
	scroll := max(0, min(r.Scrolls[n], maxScroll))
	// Keep the box the same width however it's scrolled
	visible := append([]string(nil), lines[scroll:scroll+height]...)
	visible[0] += strings.Repeat(" ", MaxWidth(code)-ansi.StringWidth(visible[0]))
	box := strings.Split(r.boxLines(visible, true, scroll+1, len(lines), color), "\n")

	// Draw a scroll thumb over the right border
	thumbH := max(1, height*height/len(lines))
	thumbTop := scroll * (height - thumbH) / maxScroll
	for i := thumbTop; i < thumbTop+thumbH; i++ {
		line := box[1+i]
		if j := strings.LastIndex(line, "│"); j >= 0 {
			box[1+i] = line[:j] + "┃" + line[j+len("│"):]
		}
	}

	r.viewports = append(r.viewports, Viewport{
		Node: n, Width: MaxWidth(box[0]), Height: len(box), Scroll: scroll, MaxScroll: maxScroll,
	})
	box[0] = fmt.Sprintf("\x1b_viewport%d\x1b\\", len(r.viewports)-1) + box[0]
	return strings.Join(box, "\n")
}

// run executes a ```run code block and returns its output. Output is cached
// so that redraws don't rerun the code.
func (r *renderer) run(n ast.Node, code string) string {
	if out, ok := r.RunCache[n]; ok {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", strings.TrimSpace(code))
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()
	out := strings.ReplaceAll(strings.TrimRight(string(output), "\n"), "\r", "")
	out = strings.ReplaceAll(out, "\t", "    ")
	if err != nil {
		out += "\n" + wrap("31;1", "22;39", err.Error())
	}
	r.RunCache[n] = out
	return out
}

// Render renders a document, sizing headings and boxes to match the widest
// element on the slide. It also returns the slide's scrollable viewports.
func Render(doc ast.Node, opts Options) (string, []Viewport) {
	r := &renderer{Options: opts}
	if r.ImageCache == nil {
		r.ImageCache = map[string]image.Image{}
	}

	// Measure everything except images, then split the remaining height
	// (minus the status bar and a margin) between the images.
	r.measuring = true
	other := strings.Count(r.blocks(doc, "\n\n"), "\n") + 1 - r.numImages
	r.measuring = false
	if r.numImages > 0 {
		r.imageRows = max(8, (r.TermH-1-2-other)/r.numImages)
	}

	r.width = r.TermW / 4
	r.width = MaxWidth(r.blocks(doc, "\n\n"))
	r.viewports = nil
	lines := strings.Split(r.blocks(doc, "\n\n"), "\n")

	// Find where each viewport ended up, and remove its marker
	for row, line := range lines {
		for _, m := range viewportMarkerRe.FindAllStringSubmatchIndex(line, -1) {
			i, _ := strconv.Atoi(line[m[2]:m[3]])
			r.viewports[i].Row = row
			r.viewports[i].Col = ansi.StringWidth(line[:m[0]])
		}
		lines[row] = viewportMarkerRe.ReplaceAllString(line, "")
	}
	return strings.Join(lines, "\n"), r.viewports
}

// Focusables returns the elements on a slide that can be activated with Enter.
func Focusables(doc ast.Node, src []byte) []ast.Node {
	var nodes []ast.Node
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			if string(n.Language(src)) == "demo" {
				nodes = append(nodes, n)
			}
		case *ast.Link, *ast.AutoLink:
			nodes = append(nodes, n)
		case *ast.Image:
			if isImage(string(n.Destination)) {
				nodes = append(nodes, n)
			}
		}
		return ast.WalkContinue, nil
	})
	return nodes
}
