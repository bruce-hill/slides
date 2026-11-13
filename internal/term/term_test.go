package term

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// waitFor waits for the terminal's screen to contain text.
func waitFor(t *testing.T, term *Term, changed chan struct{}, text string) string {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		var lines []string
		for _, line := range term.Lines(true) {
			lines = append(lines, strings.TrimRight(ansi.Strip(line), " "))
		}
		screen := strings.Join(lines, "\n")
		if strings.Contains(screen, text) {
			return screen
		}
		select {
		case <-changed:
		case <-timeout:
			t.Fatalf("terminal never showed %q:\n%s", text, screen)
		}
	}
}

func TestTerm(t *testing.T) {
	// The user's configuration is loaded, but their prompt is replaced
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DEMO_VAR", "from the environment")
	t.Setenv("HISTFILE", "/dev/null")
	t.Setenv("SHELL", "bash")
	bashrc := "alias greet='echo hello from bashrc'\nPS1='fancy prompt$ '\nPROMPT_COMMAND='echo fancy'\n"
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(bashrc), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	term, err := Start(t.TempDir(), "echo one\necho two\ngreet\necho $DEMO_VAR\n", 40, 10, func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()

	// The input is typed in, but not run until Enter is pressed
	screen := waitFor(t, term, changed, "> echo one\necho two\ngreet")
	if strings.Contains(screen, "\none\n") {
		t.Fatalf("input was run before pressing Enter:\n%s", screen)
	}
	term.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	screen = waitFor(t, term, changed, "\ntwo\nhello from bashrc\nfrom the environment\n>")
	if strings.Contains(screen, "fancy") || strings.Contains(screen, "PS1") {
		t.Errorf("the user's prompt, or the command that replaced it, is showing:\n%s", screen)
	}

	for _, line := range term.Lines(true) {
		if w := ansi.StringWidth(line); w != 40 {
			t.Errorf("line is %d columns wide, want 40: %q", w, line)
		}
	}

	term.Resize(30, 5)
	if lines := term.Lines(false); len(lines) != 5 || ansi.StringWidth(lines[0]) != 30 {
		t.Errorf("after resizing to 30x5, got %d lines of width %d", len(lines), ansi.StringWidth(lines[0]))
	}

	term.Paste("exit")
	term.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	timeout := time.After(5 * time.Second)
	for !term.Exited() {
		select {
		case <-changed:
		case <-timeout:
			t.Fatal("shell didn't exit")
		}
	}
}
