package ui

import (
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
