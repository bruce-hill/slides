package highlight

import (
	"strings"
	"testing"
)

func TestTomoLexer(t *testing.T) {
	for _, name := range []string{"tomo", "tm"} {
		if l := LexerFor(name); l == nil || l.Config().Name != "Tomo" {
			t.Fatalf("LexerFor(%q) = %v, want the Tomo lexer", name, l)
		}
	}
	out := Highlight("func main()\n    say(\"hi $(x)\")\n", LexerFor("tomo"))
	for i, line := range strings.Split(out, "\n") {
		if strings.Count(line, "\x1b[") != 2*strings.Count(line, "\x1b[m") {
			t.Errorf("line %d has unbalanced styles: %q", i+1, line)
		}
	}
}
