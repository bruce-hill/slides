package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
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
