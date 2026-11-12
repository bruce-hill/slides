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
	wheelMsg    struct{}
)

type model struct {
	slides []deck.Slide
	index  int

	// State for the current slide
	doc        ast.Node
	focusables []ast.Node
	focus      int
	runCache   map[ast.Node]string
	scrolls    map[ast.Node]int // Scroll positions of embedded file viewports
	fileCache  map[string]string
	viewports  []render.Viewport
	lines      []string
	lineWidth  int
	scroll     int

	raw           bool
	width, height int
	start         time.Time
	prompt        promptMode
	input         textinput.Model
	search        string

	// Mouse wheel events are added up and handled together, so scrolling
	// quickly doesn't queue up more redraws than can keep up.
	wheelDelta   int
	wheelMouse   tea.Mouse
	wheelPending bool
	kitty        *render.Kitty // nil if the terminal doesn't support kitty graphics
}

// New creates a presenter for the given slides.
func New(slides []deck.Slide) tea.Model {
	input := textinput.New()
	input.Prompt = ""
	m := &model{slides: slides, start: time.Now(), input: input}
	if render.KittySupported() {
		m.kitty = render.NewKitty()
	}
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
	m.scrolls = map[ast.Node]int{}
	m.fileCache = map[string]string{}
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
	m.viewports = nil
	if m.raw {
		out = highlight.Highlight(text, highlight.LexerFor("markdown"))
	} else if strings.TrimSpace(text) != "" {
		out, m.viewports = render.Render(m.doc, render.Options{
			Src: []byte(text), Dir: m.dir(), TermW: m.width, TermH: m.height,
			Focused: m.focused(), RunCache: m.runCache, Kitty: m.kitty, Scrolls: m.scrolls, FileCache: m.fileCache,
		})
	}
	m.lines = strings.Split(out, "\n")
	m.lineWidth = render.MaxWidth(out)
	m.setScroll(m.scroll)
}

func (m *model) setScroll(scroll int) {
	m.scroll = max(0, min(scroll, len(m.lines)-(m.height-1)))
}

// offset returns where the slide's text is drawn on the screen.
func (m *model) offset() (x, y int) {
	if m.raw {
		return 0, -m.scroll
	}
	avail := m.height - 1 // The last row is the status bar
	return max(0, (m.width-m.lineWidth)/2), max(0, (avail-len(m.lines))/2) - m.scroll
}

// scrollWheel scrolls the viewport under the mouse, or the whole slide if
// there isn't one.
func (m *model) scrollWheel(mouse tea.Mouse, delta int) {
	x, y := m.offset()
	col, row := mouse.X-x, mouse.Y-y
	for _, v := range m.viewports {
		if row >= v.Row && row < v.Row+v.Height && col >= v.Col && col < v.Col+v.Width {
			if scroll := max(0, min(v.Scroll+3*delta, v.MaxScroll)); scroll != v.Scroll {
				m.scrolls[v.Node] = scroll
				m.rerender()
			}
			return
		}
	}
	m.setScroll(m.scroll + delta)
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

// resetImages deletes the images sent to the terminal and resends them, which
// is needed after the terminal has been used by another program.
func (m *model) resetImages() tea.Cmd {
	if m.kitty == nil {
		return nil
	}
	deleted := m.kitty.Clear()
	m.rerender()
	return tea.Raw(deleted)
}

func (m *model) quit() tea.Cmd {
	if m.kitty == nil {
		return tea.Quit
	}
	return tea.Sequence(tea.Raw(m.kitty.Clear()), tea.Quit)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	if m.kitty != nil {
		if images := m.kitty.Flush(); images != "" {
			cmd = tea.Sequence(cmd, tea.Raw(images))
		}
	}
	return m, cmd
}

func (m *model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if w, h, ok := cellSize(); ok && m.kitty != nil {
			m.kitty.CellW, m.kitty.CellH = w, h
		}
		m.rerender()
	case tickMsg:
		return tick()
	case demoDoneMsg:
		m.moveFocus(1)
		return m.resetImages()
	case tea.ResumeMsg:
		return m.resetImages()
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.wheelDelta--
		case tea.MouseWheelDown:
			m.wheelDelta++
		default:
			return nil
		}
		m.wheelMouse = msg.Mouse()
		if !m.wheelPending {
			m.wheelPending = true
			return func() tea.Msg { return wheelMsg{} }
		}
	case wheelMsg:
		m.scrollWheel(m.wheelMouse, m.wheelDelta)
		m.wheelDelta, m.wheelPending = 0, false
	case tea.KeyPressMsg:
		if m.prompt != noPrompt {
			return m.updatePrompt(msg)
		}
		switch key := msg.String(); key {
		case "q", "ctrl+c":
			return m.quit()
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
			return tea.Suspend
		case "enter":
			return m.activate()
		case "tab":
			m.moveFocus(1)
		case "shift+tab":
			m.moveFocus(-1)
		case "`":
			m.raw = !m.raw
			m.rerender()
		case "/":
			return m.openPrompt(searchPrompt, "")
		case "n":
			m.findSlide(1)
		case "p":
			m.findSlide(-1)
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			return m.openPrompt(gotoPrompt, key)
		}
	}
	return nil
}

func (m *model) View() tea.View {
	rows := make([]string, m.height)
	x, y := m.offset()
	for i := range m.height - 1 {
		if j := i - y; j >= 0 && j < len(m.lines) {
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
