package term

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

func TestAnswerQueries(t *testing.T) {
	for _, tc := range []struct{ name, query, reply string }{
		{"XTVERSION", "\x1b[>0q", "\x1bP>|slides\x1b\\"},
		{"DA3", "\x1b[=c", "\x1bP!|00000000\x1b\\"},
		{"window size", "\x1b[14t", "\x1b[4;480;800t"},
		{"cell size", "\x1b[16t", "\x1b[6;20;10t"},
		{"text area size", "\x1b[18t", "\x1b[8;24;80t"},
		{"XTGETTCAP", "\x1bP+q544e\x1b\\", "\x1bP0+r\x1b\\"},
		{"DECRQSS", "\x1bP$qm\x1b\\", "\x1bP0$r\x1b\\"},
		{"palette", "\x1b]4;1;?\a", "\x1b]4;1;rgb:8080/0000/0000\a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := vt.NewEmulator(80, 24)
			answerQueries(e)
			got := make(chan string, 1)
			go func() {
				buf := make([]byte, 256)
				n, _ := e.Read(buf)
				got <- string(buf[:n])
			}()
			e.WriteString(tc.query)
			select {
			case reply := <-got:
				if !strings.HasPrefix(reply, tc.reply) {
					t.Errorf("reply = %q, want %q", reply, tc.reply)
				}
			case <-time.After(time.Second):
				t.Errorf("no reply")
			}
		})
	}
}
