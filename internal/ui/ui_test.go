package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"

	"github.com/bruce-hill/slides/internal/deck"
	"github.com/bruce-hill/slides/internal/render"
)

func newModel(t *testing.T, text string) *model {
	t.Helper()
	m := New([]deck.Slide{{Filename: "test.md", Text: text}}).(*model)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func TestKittyDetection(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KONSOLE_VERSION", "")
	m := newModel(t, "# Title")
	if len(m.lines) > 1 {
		t.Errorf("slide was drawn before the terminal answered the kitty graphics query")
	}
	m.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: render.KittyQueryID}, Payload: []byte("OK")})
	m.Update(uv.PrimaryDeviceAttributesEvent{62, 22})
	if m.kitty == nil {
		t.Errorf("kitty graphics weren't used after the terminal said it supports them")
	}
	if len(m.lines) <= 1 {
		t.Errorf("slide wasn't drawn after the terminal answered")
	}

	// Terminals without kitty graphics only answer the device attributes request
	m = newModel(t, "# Title")
	m.Update(uv.PrimaryDeviceAttributesEvent{62, 22})
	if m.kitty != nil {
		t.Errorf("kitty graphics were used without the terminal supporting them")
	}

	// WezTerm supports kitty graphics, but not Unicode placeholders
	t.Setenv("TERM_PROGRAM", "WezTerm")
	m = newModel(t, "# Title")
	m.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: render.KittyQueryID}, Payload: []byte("OK")})
	m.Update(uv.PrimaryDeviceAttributesEvent{62, 22})
	if m.kitty != nil {
		t.Errorf("kitty graphics were used in a terminal without Unicode placeholders")
	}

	// Terminals that never answer
	m = newModel(t, "# Title")
	m.Update(queryTimeoutMsg{})
	if m.kitty != nil || len(m.lines) <= 1 {
		t.Errorf("slide wasn't drawn without kitty graphics after the query timed out")
	}
}

func TestKeys(t *testing.T) {
	long := "# Long\n\n" + strings.Repeat("line\n\n", 40)
	m := New([]deck.Slide{{Filename: "a.md", Text: long}, {Filename: "a.md", Text: "two"}, {Filename: "a.md", Text: "three"}}).(*model)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(queryTimeoutMsg{})
	press := func(key string) {
		code, text := []rune(key)[0], key
		m.Update(tea.KeyPressMsg{Code: code, Text: text})
	}
	press("j")
	press("j")
	press("k")
	if m.index != 0 || m.scroll != 1 {
		t.Errorf("j j k: slide %d scrolled %d, want slide 0 scrolled 1", m.index, m.scroll)
	}
	press("l")
	press("l")
	press("h")
	if m.index != 1 {
		t.Errorf("l l h: slide %d, want 1", m.index)
	}
	press("G")
	if m.index != 2 {
		t.Errorf("G: slide %d, want 2", m.index)
	}
	press("g")
	if m.index != 0 {
		t.Errorf("g: slide %d, want 0", m.index)
	}
}

func TestTyping(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // Without the user's .bashrc
	t.Setenv("SHELL", "bash")
	m := New([]deck.Slide{{Filename: "a.md", Text: "```demo\necho hi\n```"}, {Filename: "a.md", Text: "two"}}).(*model)
	defer m.quit()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(queryTimeoutMsg{})
	if len(m.slideTerms) != 1 {
		t.Fatalf("demo has %d terminals, want 1", len(m.slideTerms))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.typing {
		t.Fatalf("Enter didn't start typing in the demo's terminal")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil || m.index != 0 {
		t.Errorf("q didn't go to the terminal")
	}
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.index != 0 {
		t.Errorf("l changed slides while typing")
	}
	m.Update(tea.KeyPressMsg{Code: ']', Mod: tea.ModCtrl})
	if m.typing {
		t.Fatalf("Ctrl-] didn't stop typing")
	}
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if m.index != 0 || m.slideTerms[m.focusables[0]] == nil || len(m.terms) != 1 {
		t.Errorf("demo's terminal wasn't kept after leaving the slide and coming back")
	}
}

func TestZoom(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // Without the user's .bashrc
	t.Setenv("SHELL", "bash")
	m := New([]deck.Slide{{Filename: "a.md", Text: "# Title\n\n```demo height=5\necho hi\n```"}}).(*model)
	defer m.quit()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(queryTimeoutMsg{})
	m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if !m.typing || !m.zoomed {
		t.Fatalf("z didn't start typing in a zoomed terminal")
	}
	if lines := m.typingTerm().Lines(false); len(lines) != 29 || ansi.StringWidth(lines[0]) != 100 {
		t.Errorf("zoomed terminal is %dx%d, want 100x29", ansi.StringWidth(lines[0]), len(lines))
	}
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 20})
	if lines := m.typingTerm().Lines(false); len(lines) != 19 || ansi.StringWidth(lines[0]) != 90 {
		t.Errorf("zoomed terminal is %dx%d after resizing, want 90x19", ansi.StringWidth(lines[0]), len(lines))
	}
	m.Update(tea.KeyPressMsg{Code: ']', Mod: tea.ModCtrl})
	if m.typing || m.zoomed {
		t.Fatalf("Ctrl-] didn't stop typing and zooming")
	}
	if lines := m.slideTerms[m.focused()].Lines(false); len(lines) != 5 {
		t.Errorf("terminal is %d rows after unzooming, want 5", len(lines))
	}
}

func TestDemosStartEarly(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // Without the user's .bashrc
	t.Setenv("SHELL", "bash")
	m := New([]deck.Slide{{Filename: "a.md", Text: "one"}, {Filename: "a.md", Text: "```demo\necho hi\n```\n\n```demo\necho bye\n```"}}).(*model)
	defer m.quit()
	if len(m.terms) != 2 {
		t.Fatalf("%d demo shells were started, want 2 before their slide is shown", len(m.terms))
	}
	first := m.terms[termKey{1, 0}]
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(queryTimeoutMsg{})
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if len(m.slideTerms) != 2 || m.slideTerms[m.focusables[0]] != first || len(m.terms) != 2 {
		t.Errorf("the demo shells started early weren't used on their slide")
	}
}
