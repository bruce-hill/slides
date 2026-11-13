package term

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// Size of a character cell in pixels that's reported to programs that ask
const cellW, cellH = 10, 20

// answerQueries answers the questions that programs ask terminals which the
// emulator doesn't answer on its own. Programs often wait a second or more
// for an answer before giving up, which slows down shell startup.
func answerQueries(e *vt.Emulator) {
	reply := func(s string) bool {
		e.SendText(s)
		return true
	}
	// XTVERSION: the terminal's name and version
	e.RegisterCsiHandler(ansi.Command('>', 0, 'q'), func(params ansi.Params) bool {
		return reply("\x1bP>|slides\x1b\\")
	})
	// DA3: the terminal's unit ID
	e.RegisterCsiHandler(ansi.Command('=', 0, 'c'), func(params ansi.Params) bool {
		return reply("\x1bP!|00000000\x1b\\")
	})
	// XTWINOPS: the size of the terminal and its cells
	e.RegisterCsiHandler(ansi.Command(0, 0, 't'), func(params ansi.Params) bool {
		switch op, _, _ := params.Param(0, 0); op {
		case 14:
			return reply(fmt.Sprintf("\x1b[4;%d;%dt", e.Height()*cellH, e.Width()*cellW))
		case 16:
			return reply(fmt.Sprintf("\x1b[6;%d;%dt", cellH, cellW))
		case 18:
			return reply(fmt.Sprintf("\x1b[8;%d;%dt", e.Height(), e.Width()))
		}
		return false
	})
	// XTGETTCAP and DECRQSS: terminfo capabilities and settings, which are
	// all reported as unknown
	e.RegisterDcsHandler(ansi.Command(0, '+', 'q'), func(params ansi.Params, data []byte) bool {
		return reply("\x1bP0+r\x1b\\")
	})
	e.RegisterDcsHandler(ansi.Command(0, '$', 'q'), func(params ansi.Params, data []byte) bool {
		return reply("\x1bP0$r\x1b\\")
	})
	// Colors in the 256 color palette
	e.RegisterOscHandler(4, func(data []byte) bool {
		// The data is "4;index;?" with any number of index;? pairs
		parts := bytes.Split(data, []byte(";"))
		for i := 1; i+1 < len(parts); i += 2 {
			index, err := strconv.Atoi(string(parts[i]))
			if err != nil || index < 0 || index > 255 || string(parts[i+1]) != "?" {
				continue
			}
			r, g, b, _ := ansi.IndexedColor(index).RGBA()
			reply(fmt.Sprintf("\x1b]4;%d;rgb:%04x/%04x/%04x\a", index, r, g, b))
		}
		return true
	})
}
