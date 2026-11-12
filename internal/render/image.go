package render

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

func isImage(path string) bool {
	return slices.Contains([]string{".png", ".jpg", ".jpeg", ".bmp", ".gif", ".webp"}, strings.ToLower(filepath.Ext(path)))
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// imageToText renders an image as truecolor half-block characters, so each
// character cell holds two vertically stacked pixels.
func imageToText(img image.Image, cols, rows int) string {
	dst := image.NewRGBA(image.Rect(0, 0, cols, rows*2))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	var sb strings.Builder
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			top, bot := dst.RGBAAt(x, 2*y), dst.RGBAAt(x, 2*y+1)
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", top.R, top.G, top.B, bot.R, bot.G, bot.B)
		}
		sb.WriteString("\x1b[m")
		if y < rows-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
