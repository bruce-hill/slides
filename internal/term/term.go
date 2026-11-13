// Package term runs shells in virtual terminals that are drawn on slides.
package term

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Term is a shell running in a virtual terminal.
type Term struct {
	mu     sync.Mutex
	emu    *vt.Emulator
	pty    *os.File
	cmd    *exec.Cmd
	exited bool
	setUp  bool // Whether the prompt has been set
	pasted bool // Whether the input has been pasted, after which the screen is shown
}

// promptSetup is a command for each shell that replaces the user's prompt
// with a bold green ">", so demos don't show a big complicated prompt, then
// clears the screen. The leading space keeps it out of the history.
var promptSetup = map[string]string{
	"bash": ` PS1='\[\e[1;32m\]>\[\e[m\] ' PS2=; unset PROMPT_COMMAND; printf '\e[H\e[2J\e[3J'`,
	"zsh":  ` PROMPT='%B%F{green}>%f%b ' RPROMPT= PROMPT2=; precmd_functions=(); printf '\e[H\e[2J\e[3J'`,
	"fish": ` function fish_prompt; set_color -o green; echo -n '> '; set_color normal; end; function fish_right_prompt; end; printf '\e[H\e[2J\e[3J'`,
}

// Start starts the user's shell in dir in a cols x rows terminal, with input
// typed at its prompt but not run. notify is called from another goroutine
// whenever the screen changes.
func Start(dir, input string, cols, rows int, notify func()) (*Term, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Env = env()
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}

	t := &Term{emu: vt.NewEmulator(cols, rows), pty: f, cmd: cmd}
	setup, ok := promptSetup[filepath.Base(shell)]
	t.setUp = !ok
	input = strings.TrimRight(input, "\n")
	// Shells turn on bracketed paste when they're ready for input at each
	// prompt. At the first one, the prompt is set up, and at the next one,
	// the input is pasted, which lets multiple lines be typed without
	// running each one.
	t.emu.SetCallbacks(vt.Callbacks{EnableMode: func(mode ansi.Mode) {
		switch {
		case mode != ansi.ModeBracketedPaste || t.pasted:
		case !t.setUp:
			t.setUp = true
			t.emu.SendText(setup + "\r")
		default:
			t.pasted = true
			t.emu.Paste(input)
		}
	}})
	// Shells without bracketed paste just get everything typed in
	time.AfterFunc(3*time.Second, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if !t.setUp {
			t.setUp = true
			t.emu.SendText(setup + "\r")
		}
		if !t.pasted {
			t.pasted = true
			t.emu.SendText(input)
		}
	})

	// The emulator's replies to the shell, like keys and cursor positions
	go io.Copy(f, t.emu)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := f.Read(buf)
			t.mu.Lock()
			if n > 0 {
				t.emu.Write(buf[:n])
			}
			if err != nil {
				t.exited = true
			}
			t.mu.Unlock()
			notify()
			if err != nil {
				cmd.Wait()
				return
			}
		}
	}()
	return t, nil
}

// env is the shell's environment, without the variables that describe the
// real terminal.
func env() []string {
	var vars []string
	for _, v := range os.Environ() {
		name, _, _ := strings.Cut(v, "=")
		switch name {
		case "TERM", "COLORTERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION", "KITTY_WINDOW_ID", "TMUX", "TMUX_PANE", "COLUMNS", "LINES":
		default:
			vars = append(vars, v)
		}
	}
	return append(vars, "TERM=xterm-256color", "COLORTERM=truecolor")
}

// SendKey sends a key press to the shell.
func (t *Term) SendKey(k uv.KeyEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emu.SendKey(k)
}

// SendMouse sends a mouse event to the shell, at a position relative to the
// terminal's top left corner. It's ignored unless the program in the
// terminal has asked for mouse events.
func (t *Term) SendMouse(m uv.MouseEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emu.SendMouse(m)
}

// Paste pastes text into the shell.
func (t *Term) Paste(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emu.Paste(text)
}

// Resize changes the size of the terminal.
func (t *Term) Resize(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cols == t.emu.Width() && rows == t.emu.Height() {
		return
	}
	t.emu.Resize(cols, rows)
	pty.Setsize(t.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// Exited reports whether the shell has exited.
func (t *Term) Exited() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exited
}

// Lines returns the terminal's screen as lines of styled text that each
// stand on their own, with the cursor drawn if showCursor is set.
func (t *Term) Lines(showCursor bool) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cols := t.emu.Width()
	if !t.pasted {
		// Hide the user's prompt until it's been replaced
		return slices.Repeat([]string{strings.Repeat(" ", cols)}, t.emu.Height())
	}
	lines := strings.Split(t.emu.Render(), "\n")
	for i, line := range lines {
		if w := ansi.StringWidth(line); w < cols {
			line += "\x1b[m" + strings.Repeat(" ", cols-w)
		}
		lines[i] = line + "\x1b[m" + ansi.ResetHyperlink()
	}
	if cur := t.emu.CursorPosition(); showCursor && !t.exited && !t.emu.CursorHidden() && cur.Y >= 0 && cur.Y < len(lines) {
		line := lines[cur.Y]
		lines[cur.Y] = ansi.Truncate(line, cur.X, "") + "\x1b[7m" + ansi.Strip(ansi.Cut(line, cur.X, cur.X+1)) + "\x1b[m" +
			ansi.TruncateLeft(line, cur.X+1, "")
	}
	return lines
}

// Close stops the shell.
func (t *Term) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Emulator.Close() isn't safe to call while the input is being read, but
	// closing the input pipe ends the goroutine that reads it.
	t.emu.InputPipe().(io.Closer).Close()
	t.pty.Close()
	if t.cmd.Process != nil {
		t.cmd.Process.Signal(syscall.SIGHUP)
	}
}
