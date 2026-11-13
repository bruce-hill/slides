// Package term runs shells in virtual terminals that are drawn on slides.
package term

import (
	"fmt"
	"io"
	"log"
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
	mu      sync.Mutex
	emu     *vt.Emulator
	pty     *os.File
	cmd     *exec.Cmd
	exited  bool
	setUp   bool // Whether the prompt setup has been typed in
	entered bool // Whether Enter has been pressed to run it
	pasted  bool // Whether the input has been pasted, after which the screen is shown
}

// readyOSC is a private OSC sequence that the shell prints once its prompt
// has been replaced.
const readyOSC = 7337

// bashSetup replaces the user's prompt with a bold green ">", so demos don't
// show a big complicated prompt. It runs as the PROMPT_COMMAND before the
// first prompt, after the user's configuration has loaded, and then removes
// itself. It also clears the screen, and says that the prompt is ready.
const bashSetup = `PS1='\[\e[1;32m\]>\[\e[m\] ' PS2=; unset PROMPT_COMMAND; printf '\e[H\e[2J\e[3J\e]7337\a'`

// typedSetup does the same as bashSetup for shells without a PROMPT_COMMAND,
// or when the user's configuration replaced it, as a command that's typed in
// at the first prompt. The leading space keeps it out of the history.
var typedSetup = map[string]string{
	"bash": " " + bashSetup,
	"zsh":  ` PROMPT='%B%F{green}>%f%b ' RPROMPT= PROMPT2=; precmd_functions=(); printf '\e[H\e[2J\e[3J\e]7337\a'`,
	"fish": ` function fish_prompt; set_color -o green; echo -n '> '; set_color normal; end; function fish_right_prompt; end; printf '\e[H\e[2J\e[3J\e]7337\a'`,
}

// debugLog logs when each demo's shell becomes ready, and the escape
// sequences that the emulator doesn't handle, to the file named by
// $SLIDES_DEBUG_LOG. The shell's output isn't logged.
var debugLog = func() *log.Logger {
	if path := os.Getenv("SLIDES_DEBUG_LOG"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			return log.New(f, "", log.Lmicroseconds)
		}
	}
	return log.New(io.Discard, "", 0)
}()

type loggerFunc func(format string, args ...any)

func (f loggerFunc) Printf(format string, args ...any) { f(format, args...) }

// Start starts the user's shell in dir in a cols x rows terminal, with input
// typed at its prompt but not run. notify is called from another goroutine
// whenever the screen changes.
func Start(dir, input string, cols, rows int, notify func()) (*Term, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	name := filepath.Base(shell)
	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Env = env()
	if name == "bash" {
		cmd.Env = append(cmd.Env, "PROMPT_COMMAND="+bashSetup)
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}

	t := &Term{emu: vt.NewEmulator(cols, rows), pty: f, cmd: cmd}
	start := time.Now()
	logf := func(format string, args ...any) {
		debugLog.Printf("shell %d at %.3fs: %s", cmd.Process.Pid, time.Since(start).Seconds(), fmt.Sprintf(format, args...))
	}
	logf("started %s", shell)
	t.emu.SetLogger(loggerFunc(func(format string, args ...any) {
		// Mode changes are logged too, which aren't interesting
		if strings.HasPrefix(format, "unhandled") {
			logf(format, args...)
		}
	}))
	answerQueries(t.emu)
	setup, knownShell := typedSetup[name]
	typeSetup := func() {
		if !t.setUp && !t.pasted {
			logf("typing in the prompt setup")
			t.setUp = true
			t.emu.SendText(setup)
			// Enter is pressed separately, because ble.sh inserts a newline
			// instead of running the command when it comes with a lot of input
			time.AfterFunc(200*time.Millisecond, func() {
				t.mu.Lock()
				defer t.mu.Unlock()
				t.entered = true
				t.emu.SendText("\r")
			})
		}
	}
	input = strings.TrimRight(input, "\n")
	// The input is pasted, which lets multiple lines be typed without running
	// each one. The paste markers are always sent, because the shell may not
	// have turned on bracketed paste yet when the prompt is ready.
	paste := func() {
		if !t.pasted {
			logf("pasting the demo's code")
			t.pasted = true
			t.emu.SendText(ansi.BracketedPasteStart + input + ansi.BracketedPasteEnd)
		}
	}
	t.emu.RegisterOscHandler(readyOSC, func([]byte) bool {
		if t.setUp && !t.entered {
			// The PROMPT_COMMAND ran after all, but the prompt setup has
			// been typed in, and will say when it's ready once it runs
			logf("the prompt was set up before the typed setup ran")
			return true
		}
		// The prompt is ready, but until the shell's line editor starts
		// reading input, the terminal would echo the input back as is
		logf("the prompt is set up")
		go func() {
			for start := time.Now(); !lineEditing(f) && time.Since(start) < 5*time.Second; {
				time.Sleep(5 * time.Millisecond)
			}
			t.mu.Lock()
			paste()
			t.mu.Unlock()
			notify()
		}()
		return true
	})
	// Shells turn on bracketed paste when they're ready for input, which is
	// when the prompt setup can be typed in, or for other shells, when the
	// input can be pasted.
	t.emu.SetCallbacks(vt.Callbacks{EnableMode: func(mode ansi.Mode) {
		if mode == ansi.ModeBracketedPaste {
			logf("bracketed paste turned on")
		}
		switch {
		case mode != ansi.ModeBracketedPaste:
		case !knownShell:
			paste()
		case name == "bash":
			// The PROMPT_COMMAND says when the prompt is ready within a few
			// milliseconds, unless the user's configuration replaced it.
			// (Typing it in isn't the first choice, because ble.sh doesn't
			// run commands typed in while it's starting.)
			time.AfterFunc(250*time.Millisecond, func() {
				t.mu.Lock()
				defer t.mu.Unlock()
				typeSetup()
			})
		default:
			typeSetup()
		}
	}})
	// In case the shell's configuration replaced the PROMPT_COMMAND, or the
	// shell never turns on bracketed paste
	time.AfterFunc(5*time.Second, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if !t.pasted {
			logf("gave up waiting for the prompt, pasting anyway")
			t.pasted = true
			t.emu.Paste(input)
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
