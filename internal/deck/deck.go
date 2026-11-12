// Package deck loads slides from files.
package deck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Slide struct {
	Filename string
	Text     string
}

// Dir returns the directory that the slide's relative paths are resolved from.
func (s Slide) Dir() string { return filepath.Dir(s.Filename) }

var (
	separatorRe = regexp.MustCompile(`^-{3,}\s*$`)
	fenceRe     = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
)

// Split splits markdown text on lines of three or more dashes, ignoring any
// that appear inside of fenced code blocks.
func Split(text string) []string {
	var slides, cur []string
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		m := fenceRe.FindStringSubmatch(line)
		if fence == "" {
			if separatorRe.MatchString(line) {
				slides = append(slides, strings.TrimSpace(strings.Join(cur, "\n")))
				cur = nil
				continue
			}
			// Backtick fences can't have backticks in their info string
			if m != nil && !(m[1][0] == '`' && strings.Contains(m[2], "`")) {
				fence = m[1]
			}
		} else if m != nil && m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(m[2]) == "" {
			fence = ""
		}
		cur = append(cur, line)
	}
	return append(slides, strings.TrimSpace(strings.Join(cur, "\n")))
}

// Load reads the slides from a file. Markdown files are split into slides,
// and any other file is shown as a single code slide.
func Load(filename string) ([]Slide, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	text := string(data)
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	var slides []Slide
	switch ext {
	case "slides", "md", "markdown", "txt":
		if strings.HasPrefix(text, "#!") {
			_, text, _ = strings.Cut(text, "\n")
		}
		for _, s := range Split(text) {
			slides = append(slides, Slide{filename, s})
		}
	default:
		fence := "```"
		for strings.Contains(text, fence) {
			fence += "`"
		}
		slides = append(slides, Slide{filename, fence + ext + "\n" + strings.TrimSpace(text) + "\n" + fence})
	}
	return slides, nil
}
