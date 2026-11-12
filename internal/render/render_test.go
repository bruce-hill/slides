package render

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark/ast"
)

func TestRender(t *testing.T) {
	src := []byte("# Title\n\nSome *text* and a [link](https://example.com).\n\n- one\n- two\n\n```demo\necho hi\n```\n")
	doc := Parse(src)
	focusables := Focusables(doc, src)
	if len(focusables) != 2 {
		t.Fatalf("got %d focusables, want 2 (a link and a demo)", len(focusables))
	}
	out := Render(doc, Options{Src: src, TermW: 80, TermH: 24, Focused: focusables[1], RunCache: map[ast.Node]string{}})
	plain := ansi.Strip(out)
	for _, want := range []string{"Title", "Some text and a link.", "◆ one", "Demo (press Enter to run)", "$ echo hi"} {
		if !strings.Contains(plain, want) {
			t.Errorf("output is missing %q:\n%s", want, plain)
		}
	}
	// Headings and boxes are stretched to the width of the widest element
	lines := strings.Split(plain, "\n")
	if w := ansi.StringWidth(lines[0]); w != MaxWidth(out) {
		t.Errorf("heading width = %d, want %d", w, MaxWidth(out))
	}
}

func TestRun(t *testing.T) {
	src := []byte("```run\necho hello\n```\n")
	out := ansi.Strip(Render(Parse(src), Options{Src: src, Dir: ".", TermW: 80, TermH: 24, RunCache: map[ast.Node]string{}}))
	if !strings.Contains(out, "│ hello") {
		t.Errorf("run block output is missing:\n%s", out)
	}
}

func TestKittyPlaceholder(t *testing.T) {
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
	out := Render(Parse(src), Options{Src: src, Dir: dir, TermW: 100, TermH: 40, RunCache: map[ast.Node]string{}})
	if lines := strings.Count(out, "\n") + 1; lines > 39 {
		t.Errorf("slide is %d lines tall, which doesn't fit in a 40 line terminal", lines)
	}
}
