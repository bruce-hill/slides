// Package render renders markdown slides as styled terminal text.
package render

import (
	"context"
	"fmt"
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
	Focused      ast.Node            // The focused demo, link, or image
	RunCache     map[ast.Node]string // Output of ```run blocks
	Kitty        *Kitty              // If set, images use the kitty graphics protocol
}

type renderer struct {
	Options
	width int // Width that headings and boxes expand to fill
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
	numW := len(strconv.Itoa(len(lines)))
	bar := lipgloss.NewStyle().Foreground(color).Render("│")
	for i, line := range lines {
		if lineNumbers {
			lines[i] = fmt.Sprintf("\x1b[2m%*d\x1b[22m%s %s", numW, i+1, bar, line)
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
		return strings.TrimSpace(r.inlines(n))
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

func (r *renderer) image(n *ast.Image) string {
	path := r.resolve(string(n.Destination))
	alt := plainText(n, r.Src)
	if isImage(path) {
		img, err := loadImage(path)
		if err != nil {
			return wrap("31;1", "22;39", fmt.Sprintf("<Couldn't load image %s: %v>", n.Destination, err))
		}
		// Half-block characters show two square pixels per cell
		cellW, cellH := 1, 2
		if r.Kitty != nil {
			cellW, cellH = r.Kitty.CellW, r.Kitty.CellH
		}
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		rowsFor := func(cols int) int { return cols * cellW * h / (w * cellH) }
		colsFor := func(rows int) int { return rows * cellH * w / (h * cellW) }

		cols := min(w/cellW, max(40, 2*r.TermW/3))
		rows := rowsFor(cols)
		if maxRows := max(20, 2*r.TermH/3); rows > maxRows {
			rows = maxRows
			cols = colsFor(rows)
		}
		if m := widthOptRe.FindStringSubmatch(alt); m != nil {
			pct, _ := strconv.Atoi(m[1])
			cols = r.TermW * pct / 100
			rows = rowsFor(cols)
		}
		if m := heightOptRe.FindStringSubmatch(alt); m != nil {
			pct, _ := strconv.Atoi(m[1])
			rows = r.TermH * pct / 100
			cols = colsFor(rows)
		}
		cols, rows = max(cols, 1), max(rows, 1)

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
	contents, err := os.ReadFile(path)
	if err != nil {
		return wrap("31;1", "22;39", fmt.Sprintf("<File not found: %s>", n.Destination))
	}
	title := r.inlines(n)
	if title == "" {
		title = path
	}
	code := highlight.Highlight(string(contents), highlight.LexerFor(strings.TrimPrefix(filepath.Ext(path), ".")))
	return r.heading(title, cyan) + "\n" + r.box(code, true, cyan)
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
// element on the slide.
func Render(doc ast.Node, opts Options) string {
	r := &renderer{Options: opts}
	r.width = r.TermW / 4
	r.width = MaxWidth(r.blocks(doc, "\n\n"))
	return r.blocks(doc, "\n\n")
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
