package render

import (
	"encoding/base64"
	"fmt"
	"image"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	"golang.org/x/image/draw"
)

// KittySupported reports whether the terminal supports the kitty graphics
// protocol. Multiplexers like tmux don't pass the protocol through.
func KittySupported() bool {
	return (os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "") && os.Getenv("TMUX") == ""
}

// Kitty displays images with the kitty graphics protocol. Images are sent to
// the terminal once, then drawn with Unicode placeholder characters, which
// behave like ordinary text, so they can be centered and scrolled.
type Kitty struct {
	// Size of a character cell in pixels
	CellW, CellH int

	ids     map[kittyKey]int
	nextID  int
	pending strings.Builder
}

type kittyKey struct {
	path       string
	cols, rows int
}

func NewKitty() *Kitty {
	// Use a random range of IDs to avoid clobbering other programs' images
	return &Kitty{CellW: 1, CellH: 2, ids: map[kittyKey]int{}, nextID: 1 + rand.IntN(1<<23)}
}

// Placeholder returns text that displays an image in a cols x rows area,
// queueing the image to be sent to the terminal if it hasn't been yet.
func (k *Kitty) Placeholder(path string, img image.Image, cols, rows int) string {
	cols, rows = min(cols, 297), min(rows, 297) // Limited by available diacritics
	key := kittyKey{path, cols, rows}
	id, ok := k.ids[key]
	if !ok {
		id = k.nextID
		k.nextID++
		k.ids[key] = id
		k.transmit(id, img, cols, rows)
	}

	var sb strings.Builder
	for row := range rows {
		if row > 0 {
			sb.WriteByte('\n')
		}
		// The image ID is encoded in the foreground color
		fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", (id>>16)&0xff, (id>>8)&0xff, id&0xff)
		for col := range cols {
			sb.WriteRune(kitty.Placeholder)
			sb.WriteRune(kitty.Diacritic(row))
			sb.WriteRune(kitty.Diacritic(col))
		}
		sb.WriteString("\x1b[39m")
	}
	return sb.String()
}

func (k *Kitty) transmit(id int, img image.Image, cols, rows int) {
	// Don't send more pixels than will be displayed
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if maxW, maxH := cols*k.CellW, rows*k.CellH; w > maxW || h > maxH {
		scale := min(float64(maxW)/float64(w), float64(maxH)/float64(h))
		dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = dst
	}
	opts := &kitty.Options{
		Action:           kitty.TransmitAndPut,
		ID:               id,
		Columns:          cols,
		Rows:             rows,
		VirtualPlacement: true,
		Quiet:            2,
	}
	// Sending a large image through the tty is slow for the terminal to
	// parse, so when it can read our files, just send it a file's name.
	if !isRemote() {
		if name, err := writeRGBA(img); err == nil {
			opts.Transmission, opts.Format = kitty.TempFile, kitty.RGBA
			opts.ImageWidth, opts.ImageHeight = img.Bounds().Dx(), img.Bounds().Dy()
			k.pending.WriteString(ansi.KittyGraphics([]byte(base64.StdEncoding.EncodeToString([]byte(name))), opts.Options()...))
			return
		}
	}
	opts.Transmission, opts.Format, opts.Chunk = kitty.Direct, kitty.PNG, true
	kitty.EncodeGraphics(&k.pending, img, opts)
}

func isRemote() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// writeRGBA writes an image's raw pixels to a temporary file, which the
// terminal deletes after reading.
func writeRGBA(img image.Image) (string, error) {
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Stride != 4*rgba.Rect.Dx() {
		rgba = image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	// Kitty only deletes temporary files with this in their name
	f, err := os.CreateTemp("", "tty-graphics-protocol-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(rgba.Pix); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// Flush returns the escape sequences for any images that need to be sent to
// the terminal.
func (k *Kitty) Flush() string {
	s := k.pending.String()
	k.pending.Reset()
	return s
}

// Clear returns escape sequences to delete all images from the terminal, and
// forgets them so they will be sent again if needed.
func (k *Kitty) Clear() string {
	var sb strings.Builder
	for _, id := range k.ids {
		sb.WriteString(ansi.KittyGraphics(nil, "a=d", "d=I", fmt.Sprintf("i=%d", id), "q=2"))
	}
	clear(k.ids)
	k.pending.Reset()
	return sb.String()
}
