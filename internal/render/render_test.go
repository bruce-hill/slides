package render

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark/ast"

	"github.com/bruce-hill/slides/internal/term"
)

func TestRender(t *testing.T) {
	src := []byte("# Title\n\nSome *text* and a [link](https://example.com).\n\n- one\n- two\n\n```demo\necho hi\n```\n")
	doc := Parse(src)
	focusables := Focusables(doc, src)
	if len(focusables) != 2 {
		t.Fatalf("got %d focusables, want 2 (a link and a demo)", len(focusables))
	}
	out, _ := Render(doc, Options{Src: src, TermW: 80, TermH: 24, Focused: focusables[1]})
	plain := ansi.Strip(out)
	for _, want := range []string{"Some text and a link.", "◆ one", "Demo", "$ echo hi"} {
		if !strings.Contains(plain, want) {
			t.Errorf("output is missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Title") {
		t.Errorf("the title is drawn with the rest of the slide:\n%s", plain)
	}
	// Headings and boxes are stretched to the width of the widest element
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Demo") && ansi.StringWidth(line) != MaxWidth(out) {
			t.Errorf("heading width = %d, want %d", ansi.StringWidth(line), MaxWidth(out))
		}
	}
	// The title spans the whole terminal
	title := RenderTitle(doc, Options{Src: src, TermW: 80, TermH: 24})
	if !strings.Contains(ansi.Strip(title), "Title") {
		t.Errorf("title is missing:\n%s", ansi.Strip(title))
	}
	for _, line := range strings.Split(title, "\n") {
		if w := ansi.StringWidth(line); w != 80 {
			t.Errorf("title width = %d, want 80", w)
		}
	}
}

func TestWrap(t *testing.T) {
	long := strings.Repeat("word ", 30)
	src := []byte("# " + long + "\n\n" + long + "\n\n- " + long + "\n  - " + long + "\n")
	doc := Parse(src)
	out, _ := Render(doc, Options{Src: src, TermW: 40, TermH: 24})
	if w := MaxWidth(out); w > 36 {
		t.Errorf("text is %d columns wide, which doesn't fit in a 40 column terminal with margins:\n%s", w, ansi.Strip(out))
	}
	title := RenderTitle(doc, Options{Src: src, TermW: 40, TermH: 24})
	if w := MaxWidth(title); w != 40 || strings.Count(title, "\n") < 4 {
		t.Errorf("title isn't wrapped to the whole 40 column terminal:\n%s", ansi.Strip(title))
	}
	plain := ansi.Strip(out)
	for _, want := range []string{"\nword word", "\n  ◆ word", "\n    word", "\n      ◆ word", "\n        word"} {
		if !strings.Contains(plain, want) {
			t.Errorf("wrapped text is missing %q:\n%s", want, plain)
		}
	}
}

func TestTable(t *testing.T) {
	src := []byte("| Name | Count |\n|------|------:|\n| *apples* | 3 |\n| pears | 12 |\n")
	out, _ := Render(Parse(src), Options{Src: src, TermW: 80, TermH: 24})
	plain := ansi.Strip(out)
	for _, want := range []string{"│ Name   │ Count │", "│ apples │     3 │", "│ pears  │    12 │"} {
		if !strings.Contains(plain, want) {
			t.Errorf("table is missing %q:\n%s", want, plain)
		}
	}
}

func TestTerminal(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // Without the user's .bashrc
	t.Setenv("SHELL", "bash")
	src := []byte("# Title\n\n```demo height=5\necho hi\n```\n")
	doc := Parse(src)
	n := Focusables(doc, src)[0]
	tm, err := term.Start(t.TempDir(), "echo hi", 80, 24, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	for start := time.Now(); !strings.Contains(ansi.Strip(strings.Join(tm.Lines(false), "\n")), "> echo hi"); {
		if time.Since(start) > 5*time.Second {
			t.Fatal("the demo's code was never typed in")
		}
		time.Sleep(10 * time.Millisecond)
	}

	opts := Options{Src: src, TermW: 100, TermH: 40, Focused: n, Typing: true, Terms: map[ast.Node]*term.Term{n: tm}}
	size := TermSizes(doc, opts)[n]
	tm.Resize(size.X, size.Y)
	out, viewports := Render(doc, opts)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "Ctrl-]") || !strings.Contains(plain, "┃> echo hi") {
		t.Errorf("terminal isn't drawn in a box with the code typed in:\n%s", plain)
	}
	if len(viewports) != 1 || viewports[0].Node != n || viewports[0].Height != 7 || viewports[0].Width != 80 {
		t.Errorf("viewports = %+v, want an 80x7 box (78x5 inside the border)", viewports)
	}
}

func TestKittyPlaceholder(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // The image is written to a file no terminal will delete
	k := NewKitty()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	out := k.Placeholder("img.png", img, 4, 3)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 || ansi.StringWidth(lines[0]) != 4 {
		t.Fatalf("got %d lines of width %d, want 3 lines of width 4", len(lines), ansi.StringWidth(lines[0]))
	}
	if sent := k.Flush(); !strings.Contains(sent, "U=1") || !strings.Contains(sent, "c=4,r=3") {
		t.Errorf("image wasn't sent as a 4x3 virtual placement: %q", sent[:min(len(sent), 80)])
	}
	k.Placeholder("img.png", img, 4, 3)
	if sent := k.Flush(); sent != "" {
		t.Errorf("image was sent twice")
	}
}

func TestImageSize(t *testing.T) {
	r := &renderer{Options: Options{TermW: 100, TermH: 40}, imageRows: 30}
	for _, tc := range []struct {
		alt        string
		w, h       int
		cols, rows int
	}{
		{"", 1000, 500, 80, 20},                     // Limited to 80% of the width
		{"", 1000, 2000, 30, 30},                    // Limited by the leftover height
		{"", 100, 100, 20, 10},                      // Enlarged at most 2x
		{"width=50%", 1000, 500, 50, 12},            // Width option replaces the width limit
		{"width=100% height=10%", 1000, 500, 16, 4}, // Both options fit the image in a box
	} {
		// 10x20 pixel cells
		if cols, rows := r.imageSize(tc.alt, tc.w, tc.h, 10, 20); cols != tc.cols || rows != tc.rows {
			t.Errorf("imageSize(%q, %d, %d) = %dx%d, want %dx%d", tc.alt, tc.w, tc.h, cols, rows, tc.cols, tc.rows)
		}
	}
}

func TestImageFitsSlide(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "tall.png"))
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 100, 1000)))
	f.Close()

	src := []byte("# Title\n\n![](tall.png)\n\n- one\n- two\n- three\n")
	out, _ := Render(Parse(src), Options{Src: src, Dir: dir, TermW: 100, TermH: 40})
	if lines := strings.Count(out, "\n") + 1; lines > 39 {
		t.Errorf("slide is %d lines tall, which doesn't fit in a 40 line terminal", lines)
	}
}

func TestImageCentered(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "small.png"))
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	f.Close()

	src := []byte("## A heading that is wider than the image\n\n![width=10%](small.png)\n")
	out, _ := Render(Parse(src), Options{Src: src, Dir: dir, TermW: 100, TermH: 40})
	lines := strings.Split(ansi.Strip(out), "\n")
	title, img := lines[0], lines[2]
	left, right := len(img)-len(strings.TrimLeft(img, " ")), MaxWidth(title)-ansi.StringWidth(strings.TrimRight(img, " "))
	if left == 0 || left-right > 1 || right-left > 1 {
		t.Errorf("image isn't centered under the heading (%d columns left, %d right):\n%s\n%s", left, right, title, img)
	}
}

func TestViewport(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := range 100 {
		lines = append(lines, fmt.Sprintf("line %d", i+1))
		if i == 10 {
			lines[i] += " is the longest line"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "long.txt"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	src := []byte("# Title\n\n- ![Long](long.txt)\n")
	doc := Parse(src)
	var node ast.Node
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*ast.Image); ok && entering {
			node = n
		}
		return ast.WalkContinue, nil
	})

	out, viewports := Render(doc, Options{Src: src, Dir: dir, TermW: 80, TermH: 40, Scrolls: map[ast.Node]int{node: 50}})
	if len(viewports) != 1 {
		t.Fatalf("got %d viewports, want 1", len(viewports))
	}
	v := viewports[0]
	if v.Node != node || v.Height != 22 || v.Scroll != 50 || v.MaxScroll != 80 {
		t.Errorf("viewport = %+v, want 22 rows scrolled to line 51 of 100", v)
	}
	rows := strings.Split(ansi.Strip(out), "\n")
	if top := []rune(rows[v.Row]); v.Col >= len(top) || top[v.Col] != '┌' {
		t.Errorf("viewport position (%d, %d) isn't the box's corner:\n%s", v.Row, v.Col, ansi.Strip(out))
	}
	if !strings.Contains(rows[v.Row+1], " 51│ line 51") || !strings.Contains(rows[v.Row+20], " 70│ line 70") {
		t.Errorf("viewport doesn't show lines 51-70:\n%s", ansi.Strip(out))
	}
	top, _ := Render(doc, Options{Src: src, Dir: dir, TermW: 80, TermH: 40, Scrolls: map[ast.Node]int{}})
	if MaxWidth(top) != MaxWidth(out) {
		t.Errorf("viewport width changed when scrolled: %d at the top, %d scrolled", MaxWidth(top), MaxWidth(out))
	}
	if strings.Contains(out, "\x1b_") {
		t.Errorf("viewport marker was left in the output")
	}
}

func TestReveal(t *testing.T) {
	src := []byte("# Title\n\n- one\n- two\n\nA [link](https://example.com).")
	full, _ := Render(Parse(src), Options{Src: src, TermW: 80, TermH: 24})
	reveal := strings.Index(string(src), "- two")
	doc := Parse(src)
	out, _ := Render(doc, Options{Src: src, TermW: 80, TermH: 24, Reveal: reveal})
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "◆ one") || strings.Contains(plain, "two") || strings.Contains(plain, "link") {
		t.Errorf("only the first item should be revealed:\n%s", plain)
	}
	if strings.Count(out, "\n") != strings.Count(full, "\n") || MaxWidth(out) != MaxWidth(full) {
		t.Errorf("the partly revealed slide isn't the same size as the whole slide:\n%s", plain)
	}
	focusables := Focusables(doc, src)
	if len(focusables) != 1 || Revealed(focusables[0], reveal) || !Revealed(focusables[0], 0) {
		t.Errorf("the link shouldn't be revealed yet")
	}
}
