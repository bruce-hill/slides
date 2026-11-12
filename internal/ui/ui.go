// Package ui is the interactive slide presenter.
package ui

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark/ast"

	"github.com/bruce-hill/slides/internal/deck"
	"github.com/bruce-hill/slides/internal/highlight"
	"github.com/bruce-hill/slides/internal/render"
)

type promptMode int

const (
	noPrompt promptMode = iota
	gotoPrompt
	searchPrompt
)

type (
	tickMsg     time.Time
	demoDoneMsg struct{}
)

type model struct {
	slides []deck.Slide
	index  int

	// State for the current slide
	doc        ast.Node
	focusables []ast.Node
	focus      int
	runCache   map[ast.Node]string
	lines      []string
	lineWidth  int
	scroll     int

	raw           bool
	width, height int
	start         time.Time
	prompt        promptMode
	input         textinput.Model
	search        string
}

// New creates a presenter for the given slides.
func New(slides []deck.Slide) tea.Model {
	input := textinput.New()
	input.Prompt = ""
	m := &model{slides: slides, start: time.Now(), input: input}
	m.loadSlide(0)
	return m
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) dir() string { return m.slides[m.index].Dir() }

func (m *model) loadSlide(index int) {
	m.index = max(0, min(len(m.slides)-1, index))
	src := []byte(m.slides[m.index].Text)
	m.doc = render.Parse(src)
	m.focusables = render.Focusables(m.doc, src)
	m.focus = 0
	m.runCache = map[ast.Node]string{}
	m.scroll = 0
	m.rerender()
}

func (m *model) focused() ast.Node {
	if m.focus < len(m.focusables) {
		return m.focusables[m.focus]
	}
	return nil
}

// rerender redraws the slide's content, which is only needed when something
// other than the scroll position changes.
func (m *model) rerender() {
	if m.width == 0 {
		return
	}
	text := m.slides[m.index].Text
	var out string
	if m.raw {
		out = highlight.Highlight(text, highlight.LexerFor("markdown"))
	} else if strings.TrimSpace(text) != "" {
		out = render.Render(m.doc, render.Options{
			Src: []byte(text), Dir: m.dir(), TermW: m.width, TermH: m.height,
			Focused: m.focused(), RunCache: m.runCache,
		})
	}
	m.lines = strings.Split(out, "\n")
	m.lineWidth = render.MaxWidth(out)
	m.setScroll(m.scroll)
}

func (m *model) setScroll(scroll int) {
	m.scroll = max(0, min(scroll, len(m.lines)-(m.height-1)))
}

func (m *model) findSlide(step int) {
	query := strings.ToLower(m.search)
	if query == "" {
		return
	}
	n := len(m.slides)
	for offset := 1; offset <= n; offset++ {
		i := ((m.index+step*offset)%n + n) % n
		if strings.Contains(strings.ToLower(m.slides[i].Text), query) {
			m.loadSlide(i)
			return
		}
	}
}

// demoCmd runs a demo on a cleared screen.
type demoCmd struct{ *exec.Cmd }

func (c demoCmd) Run() error {
	if c.Stdout != nil {
		io.WriteString(c.Stdout, "\x1b[2J\x1b[H")
	}
	return c.Cmd.Run()
}
func (c demoCmd) SetStdin(r io.Reader)  { c.Stdin = r }
func (c demoCmd) SetStdout(w io.Writer) { c.Stdout = w }
func (c demoCmd) SetStderr(w io.Writer) { c.Stderr = w }

func openExternal(target string) {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	cmd := exec.Command(opener, target)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

// activate runs the focused demo or opens the focused link or image.
func (m *model) activate() tea.Cmd {
	src := []byte(m.slides[m.index].Text)
	switch n := m.focused().(type) {
	case *ast.FencedCodeBlock:
		cmd := exec.Command("bash", "-c", strings.TrimSpace(render.CodeText(n, src)))
		cmd.Dir = m.dir()
		return tea.Exec(demoCmd{cmd}, func(error) tea.Msg { return demoDoneMsg{} })
	case *ast.Link:
		openExternal(string(n.Destination))
	case *ast.AutoLink:
		openExternal(string(n.URL(src)))
	case *ast.Image:
		dest := string(n.Destination)
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(m.dir(), dest)
		}
		openExternal(dest)
	default:
		return nil
	}
	m.moveFocus(1)
	return nil
}

func (m *model) moveFocus(delta int) {
	if f := m.focus + delta; f >= 0 && f < len(m.focusables) {
		m.focus = f
		m.rerender()
	}
}

func (m *model) updatePrompt(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	switch {
	case key == "enter":
		value := m.input.Value()
		if m.prompt == gotoPrompt {
			if n, err := strconv.Atoi(value); err == nil {
				m.loadSlide(n - 1)
			}
		} else {
			m.search = value
			m.findSlide(1)
		}
		m.prompt = noPrompt
		return nil
	case key == "esc" || key == "ctrl+c":
		m.prompt = noPrompt
		return nil
	case m.prompt == gotoPrompt && !(len(key) == 1 && key[0] >= '0' && key[0] <= '9') && key != "backspace":
		m.prompt = noPrompt
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *model) openPrompt(mode promptMode, initial string) tea.Cmd {
	m.prompt = mode
	m.input.SetValue(initial)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.rerender()
	case tickMsg:
		return m, tick()
	case demoDoneMsg:
		m.moveFocus(1)
		m.rerender()
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.setScroll(m.scroll - 1)
		case tea.MouseWheelDown:
			m.setScroll(m.scroll + 1)
		}
	case tea.KeyPressMsg:
		if m.prompt != noPrompt {
			return m, m.updatePrompt(msg)
		}
		switch key := msg.String(); key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "k", "backspace":
			m.loadSlide(m.index - 1)
		case "right", "j", "space":
			m.loadSlide(m.index + 1)
		case "home", "h":
			m.loadSlide(0)
		case "end", "l":
			m.loadSlide(len(m.slides) - 1)
		case "up":
			m.setScroll(m.scroll - 1)
		case "down":
			m.setScroll(m.scroll + 1)
		case "ctrl+u", "pgup":
			m.setScroll(m.scroll - 10)
		case "ctrl+d", "pgdown":
			m.setScroll(m.scroll + 10)
		case "r", "ctrl+r":
			m.runCache = map[ast.Node]string{}
			m.rerender()
		case "ctrl+z":
			return m, tea.Suspend
		case "enter":
			return m, m.activate()
		case "tab":
			m.moveFocus(1)
		case "shift+tab":
			m.moveFocus(-1)
		case "`":
			m.raw = !m.raw
			m.rerender()
		case "/":
			return m, m.openPrompt(searchPrompt, "")
		case "n":
			m.findSlide(1)
		case "p":
			m.findSlide(-1)
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			return m, m.openPrompt(gotoPrompt, key)
		}
	}
	return m, nil
}

func (m *model) View() tea.View {
	rows := make([]string, m.height)
	avail := m.height - 1 // The last row is the status bar
	x, y := 0, 0
	if !m.raw {
		x = max(0, (m.width-m.lineWidth)/2)
		y = max(0, (avail-len(m.lines))/2)
	}
	for i := range avail {
		if j := i - y + m.scroll; j >= 0 && j < len(m.lines) {
			rows[i] = strings.Repeat(" ", x) + ansi.Truncate(m.lines[j], m.width-x, "")
		}
	}

	if m.height > 0 {
		var left string
		switch m.prompt {
		case gotoPrompt:
			left = " \x1b[1mGo to slide:\x1b[22m " + m.input.View()
		case searchPrompt:
			left = " \x1b[1mSearch:\x1b[22m " + m.input.View()
		default:
			elapsed := time.Since(m.start)
			h, mins, s := int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60
			if h > 0 {
				left = fmt.Sprintf("    %d:%02d:%02d", h, mins, s)
			} else {
				left = fmt.Sprintf("    %2d:%02d", mins, s)
			}
			left = "\x1b[2m" + left + "\x1b[22m"
		}
		right := fmt.Sprintf("\x1b[2m%d/%d\x1b[22m", m.index+1, len(m.slides))
		gap := max(1, m.width-ansi.StringWidth(left)-ansi.StringWidth(right))
		rows[m.height-1] = left + strings.Repeat(" ", gap) + right
	}

	v := tea.NewView(strings.Join(rows, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
