package render

import (
	"image"
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
