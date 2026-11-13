// Package ui is the interactive slide presenter.
package ui

import (
	"fmt"
	"image"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark/ast"

	"github.com/bruce-hill/slides/internal/deck"
	"github.com/bruce-hill/slides/internal/highlight"
	"github.com/bruce-hill/slides/internal/render"
	"github.com/bruce-hill/slides/internal/term"
)

type promptMode int

const (
	noPrompt promptMode = iota
	gotoPrompt
	searchPrompt
)

type (
	tickMsg         time.Time
	errorMsg        struct{ err error }
	wheelMsg        struct{}
	queryTimeoutMsg struct{}
	termOutputMsg   struct{}
)

// termKey identifies a demo's terminal by its slide and its position on the
// slide, which stay the same when the slide is parsed again.
type termKey struct{ slide, demo int }

type model struct {
	slides []deck.Slide
	index  int

	// State for the current slide
	doc        ast.Node
	focusables []ast.Node
	focus      int
	scrolls    map[ast.Node]int // Scroll positions of embedded file viewports
	fileCache  map[string]string
	imageCache map[string]image.Image
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
	err           error     // An error shown in the status bar
	errTime       time.Time // When the error happened

	// Mouse wheel events are added up and handled together, so scrolling
	// quickly doesn't queue up more redraws than can keep up.
	wheelDelta   int
	wheelMouse   tea.Mouse
	wheelPending bool
	kitty        *render.Kitty // nil if the terminal doesn't support kitty graphics

	// Slides aren't drawn until the terminal has said whether it supports
	// kitty graphics, so images aren't drawn twice in different ways.
	querying   bool
	kittyReply bool

	// Each demo has a terminal, which keeps running after leaving its slide
	terms      map[termKey]*term.Term
	termErrs   map[termKey]error       // Why terminals couldn't be started
	slideTerms map[ast.Node]*term.Term // Terminals on the current slide
	typing     bool                    // Whether keys go to the focused terminal
	zoomed     bool                    // Whether that terminal fills the screen
	termOutput chan struct{}           // Signaled when a terminal's screen changes
}

// New creates a presenter for the given slides.
func New(slides []deck.Slide) tea.Model {
	input := textinput.New()
	input.Prompt = ""
	m := &model{
		slides: slides, start: time.Now(), input: input, querying: true,
		terms: map[termKey]*term.Term{}, termErrs: map[termKey]error{}, termOutput: make(chan struct{}, 1),
	}
	// Shells can take a while to start, so every demo's shell is started now,
	// to be ready by the time its slide is shown
	for i, slide := range slides {
		src := []byte(slide.Text)
		demo := 0
		for _, n := range render.Focusables(render.Parse(src), src) {
			if n, ok := n.(*ast.FencedCodeBlock); ok {
				m.startTerm(termKey{i, demo}, n)
				demo++
			}
		}
	}
	m.loadSlide(0)
	return m
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd {
	// In case the terminal never answers
	timeout := tea.Tick(time.Second, func(time.Time) tea.Msg { return queryTimeoutMsg{} })
	return tea.Batch(tick(), tea.Raw(render.KittyQuery), timeout, m.waitForOutput())
}

// waitForOutput waits for any terminal's screen to change. Changes that
// happen close together are drawn together.
func (m *model) waitForOutput() tea.Cmd {
	output := m.termOutput
	return func() tea.Msg {
		<-output
		time.Sleep(time.Second / 60)
		return termOutputMsg{}
	}
}

// startTerm starts a demo's terminal, with the demo's code typed in.
func (m *model) startTerm(key termKey, n *ast.FencedCodeBlock) *term.Term {
	code := render.CodeText(n, []byte(m.slides[key.slide].Text))
	t, err := term.Start(m.slides[key.slide].Dir(), code, 80, 24, func() {
		select {
		case m.termOutput <- struct{}{}:
		default:
		}
	})
	if err != nil {
		m.termErrs[key] = err
		return nil
	}
	delete(m.termErrs, key)
	m.terms[key] = t
	return t
}

// queryDone finishes checking the terminal for kitty graphics support.
func (m *model) queryDone() {
	if !m.querying {
		return
	}
	m.querying = false
	if m.kittyReply && render.KittyPlaceholders() {
		m.kitty = render.NewKitty()
		m.setCellSize()
	}
	m.rerender()
}

func (m *model) setCellSize() {
	if w, h, ok := cellSize(); ok && m.kitty != nil {
		m.kitty.CellW, m.kitty.CellH = w, h
	}
}

func (m *model) dir() string { return m.slides[m.index].Dir() }

func (m *model) loadSlide(index int) {
	m.index = max(0, min(len(m.slides)-1, index))
	src := []byte(m.slides[m.index].Text)
	m.doc = render.Parse(src)
	m.focusables = render.Focusables(m.doc, src)
	m.focus = 0
	m.scrolls = map[ast.Node]int{}
	m.fileCache = map[string]string{}
	m.imageCache = map[string]image.Image{}
	m.scroll = 0
	m.typing, m.zoomed = false, false
	m.slideTerms = map[ast.Node]*term.Term{}
	for _, n := range m.focusables {
		if n, ok := n.(*ast.FencedCodeBlock); ok {
			t := m.terms[m.demoKey(n)]
			if t == nil {
				t = m.startTerm(m.demoKey(n), n)
			}
			if t != nil {
				m.slideTerms[n] = t
			}
		}
	}
	m.rerender()
}

// slideTermErrs returns why the current slide's terminals couldn't be started.
func (m *model) slideTermErrs() map[ast.Node]error {
	errs := map[ast.Node]error{}
	for _, n := range m.focusables {
		if n, ok := n.(*ast.FencedCodeBlock); ok {
			if err := m.termErrs[m.demoKey(n)]; err != nil {
				errs[n] = err
			}
		}
	}
	return errs
}

// demoKey returns the key for a demo on the current slide.
func (m *model) demoKey(n *ast.FencedCodeBlock) termKey {
	demo := 0
	for _, f := range m.focusables {
		if f == n {
			break
		}
		if _, ok := f.(*ast.FencedCodeBlock); ok {
			demo++
		}
	}
	return termKey{m.index, demo}
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
	if m.width == 0 || m.querying {
		return
	}
	if t := m.typingTerm(); t != nil && m.zoomed {
		t.Resize(m.width, m.height-1) // Above the status bar
		return
	}
	text := m.slides[m.index].Text
	var out string
	m.viewports = nil
	if m.raw {
		out = highlight.Highlight(text, highlight.LexerFor("markdown"))
	} else if strings.TrimSpace(text) != "" {
		opts := render.Options{
			Src: []byte(text), Dir: m.dir(), TermW: m.width, TermH: m.height,
			Focused: m.focused(), Kitty: m.kitty, Scrolls: m.scrolls, FileCache: m.fileCache,
			ImageCache: m.imageCache, Terms: m.slideTerms, TermErrs: m.slideTermErrs(), Typing: m.typing,
		}
		for n, size := range render.TermSizes(m.doc, opts) {
			if t := m.slideTerms[n]; t != nil {
				t.Resize(size.X, size.Y)
			}
		}
		out, m.viewports = render.Render(m.doc, opts)
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

// viewportAt returns the viewport at a position on the screen, if any.
func (m *model) viewportAt(mouse tea.Mouse) (render.Viewport, bool) {
	x, y := m.offset()
	col, row := mouse.X-x, mouse.Y-y
	for _, v := range m.viewports {
		if row >= v.Row && row < v.Row+v.Height && col >= v.Col && col < v.Col+v.Width {
			return v, true
		}
	}
	return render.Viewport{}, false
}

// typingTerm returns the terminal that keys are going to, if any.
func (m *model) typingTerm() *term.Term {
	if !m.typing {
		return nil
	}
	return m.slideTerms[m.focused()]
}

// termMouse returns a mouse event's position inside the terminal that keys
// are going to, if it's over it.
func (m *model) termMouse(mouse tea.Mouse) (uv.Mouse, bool) {
	if m.zoomed && m.typingTerm() != nil {
		return uv.Mouse(mouse), mouse.Y < m.height-1
	}
	v, ok := m.viewportAt(mouse)
	if !ok || m.typingTerm() == nil || v.Node != m.focused() {
		return uv.Mouse{}, false
	}
	x, y := m.offset()
	// Inside the border
	return uv.Mouse{X: mouse.X - x - v.Col - 1, Y: mouse.Y - y - v.Row - 1, Button: mouse.Button, Mod: mouse.Mod}, true
}

// scrollWheel scrolls the viewport under the mouse, or the whole slide if
// there isn't one.
func (m *model) scrollWheel(mouse tea.Mouse, delta int) {
	if tm, ok := m.termMouse(mouse); ok {
		tm.Button = uv.MouseWheelDown
		if delta < 0 {
			tm.Button, delta = uv.MouseWheelUp, -delta
		}
		for range delta {
			m.typingTerm().SendMouse(uv.MouseWheelEvent(tm))
		}
		return
	}
	if v, ok := m.viewportAt(mouse); ok && m.slideTerms[v.Node] == nil {
		if scroll := max(0, min(v.Scroll+3*delta, v.MaxScroll)); scroll != v.Scroll {
			m.scrolls[v.Node] = scroll
			m.rerender()
		}
		return
	}
	m.setScroll(m.scroll + delta)
}

// click starts typing in a terminal when it's clicked, and stops when
// something else is clicked.
func (m *model) click(mouse tea.Mouse) {
	if tm, ok := m.termMouse(mouse); ok && m.zoomed {
		m.typingTerm().SendMouse(uv.MouseClickEvent(tm))
		return
	}
	v, ok := m.viewportAt(mouse)
	if t := m.slideTerms[v.Node]; ok && t != nil && !t.Exited() {
		for i, n := range m.focusables {
			if n == v.Node {
				m.focus = i
			}
		}
		m.typing = true
		m.rerender()
		if tm, ok := m.termMouse(mouse); ok {
			t.SendMouse(uv.MouseClickEvent(tm))
		}
	} else if m.typing {
		m.stopTyping()
	}
}

func (m *model) stopTyping() {
	m.typing, m.zoomed = false, false
	m.rerender()
}

// zoom starts typing in the focused demo's terminal, which fills the screen
// until typing stops.
func (m *model) zoom() tea.Cmd {
	if _, ok := m.focused().(*ast.FencedCodeBlock); !ok {
		return nil
	}
	cmd := m.activate()
	if m.typing {
		m.zoomed = true
		m.rerender()
	}
	return cmd
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

// openExternal opens a link or file in another program, and reports an error
// if it couldn't be opened.
func openExternal(target string) tea.Cmd {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return func() tea.Msg {
		if err := exec.Command(opener, target).Run(); err != nil {
			return errorMsg{fmt.Errorf("couldn't open %s: %w", target, err)}
		}
		return nil
	}
}

// activate starts typing in the focused demo's terminal (restarting it if
// it has exited), or opens the focused link or image.
func (m *model) activate() tea.Cmd {
	src := []byte(m.slides[m.index].Text)
	var cmd tea.Cmd
	switch n := m.focused().(type) {
	case *ast.FencedCodeBlock:
		t := m.slideTerms[n]
		if t == nil {
			return nil
		}
		if t.Exited() {
			t.Close()
			delete(m.terms, m.demoKey(n))
			delete(m.slideTerms, n)
			if t = m.startTerm(m.demoKey(n), n); t == nil {
				m.rerender()
				return nil
			}
			m.slideTerms[n] = t
		}
		m.typing = true
		m.rerender()
		return nil
	case *ast.Link:
		cmd = openExternal(string(n.Destination))
	case *ast.AutoLink:
		cmd = openExternal(string(n.URL(src)))
	case *ast.Image:
		dest := string(n.Destination)
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(m.dir(), dest)
		}
		cmd = openExternal(dest)
	default:
		return nil
	}
	m.moveFocus(1)
	return cmd
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
	for _, t := range m.terms {
		t.Close()
	}
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
		m.setCellSize()
		m.rerender()
	case uv.KittyGraphicsEvent:
		if msg.Options.ID == render.KittyQueryID && string(msg.Payload) == "OK" {
			m.kittyReply = true
		}
	case uv.PrimaryDeviceAttributesEvent, queryTimeoutMsg:
		m.queryDone()
	case tickMsg:
		return tick()
	case errorMsg:
		m.err, m.errTime = msg.err, time.Now()
	case termOutputMsg:
		if t := m.typingTerm(); t != nil && t.Exited() {
			m.stopTyping()
		} else if len(m.slideTerms) > 0 && !m.zoomed {
			m.rerender()
		}
		return m.waitForOutput()
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
	case tea.MouseClickMsg:
		m.click(msg.Mouse())
	case tea.MouseReleaseMsg:
		if tm, ok := m.termMouse(msg.Mouse()); ok {
			m.typingTerm().SendMouse(uv.MouseReleaseEvent(tm))
		}
	case tea.MouseMotionMsg:
		if tm, ok := m.termMouse(msg.Mouse()); ok {
			m.typingTerm().SendMouse(uv.MouseMotionEvent(tm))
		}
	case tea.PasteMsg:
		if t := m.typingTerm(); t != nil {
			t.Paste(msg.Content)
		}
	case tea.KeyPressMsg:
		if m.prompt != noPrompt {
			return m.updatePrompt(msg)
		}
		if t := m.typingTerm(); t != nil && !t.Exited() {
			if msg.String() == "ctrl+]" {
				m.stopTyping()
			} else {
				t.SendKey(uv.KeyPressEvent(msg))
			}
			return nil
		}
		if m.typing {
			m.stopTyping()
		}
		switch key := msg.String(); key {
		case "q", "ctrl+c":
			return m.quit()
		case "left", "h", "backspace":
			m.loadSlide(m.index - 1)
		case "right", "l", "space":
			m.loadSlide(m.index + 1)
		case "home", "g":
			m.loadSlide(0)
		case "end", "G":
			m.loadSlide(len(m.slides) - 1)
		case "up", "k":
			m.setScroll(m.scroll - 1)
		case "down", "j":
			m.setScroll(m.scroll + 1)
		case "ctrl+u", "pgup":
			m.setScroll(m.scroll - 10)
		case "ctrl+d", "pgdown":
			m.setScroll(m.scroll + 10)
		case "r", "ctrl+r":
			m.rerender()
			return m.resetImages()
		case "ctrl+z":
			return tea.Suspend
		case "enter":
			return m.activate()
		case "z":
			return m.zoom()
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
	if t := m.typingTerm(); t != nil && m.zoomed {
		for i, line := range t.Lines(true) {
			if i < m.height-1 {
				rows[i] = ansi.Truncate(line, m.width, "")
			}
		}
	} else {
		x, y := m.offset()
		for i := range m.height - 1 {
			if j := i - y; j >= 0 && j < len(m.lines) {
				rows[i] = strings.Repeat(" ", x) + ansi.Truncate(m.lines[j], m.width-x, "")
			}
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
			if m.zoomed {
				left += "  Demo (press Ctrl-] to return to the slides)"
			}
			left = "\x1b[2m" + left + "\x1b[22m"
			if m.err != nil && time.Since(m.errTime) < 5*time.Second {
				left += "  \x1b[31;1m" + ansi.Truncate(m.err.Error(), max(0, m.width-20), "…") + "\x1b[m"
			}
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
