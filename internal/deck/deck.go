// Package deck loads slides from files.
package deck

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type Slide struct {
	Filename string
	Text     string
	// If nonzero, only the part of the text before this offset has been
	// revealed, for slides that are shown a step at a time
	Reveal int
	Step   int // Which step of its slide this is, starting from 0
}

// Dir returns the directory that the slide's relative paths are resolved from.
func (s Slide) Dir() string { return filepath.Dir(s.Filename) }

var (
	separatorRe = regexp.MustCompile(`^-{3,}\s*$`)
	fenceRe     = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	pauseRe     = regexp.MustCompile(`^\s*<!--\s*pause\s*-->\s*$`)
)

// fences keeps track of which lines are in fenced code blocks.
type fences struct{ fence string }

// next reports whether the next line is part of a fenced code block,
// including the lines that open and close it.
func (f *fences) next(line string) bool {
	m := fenceRe.FindStringSubmatch(strings.TrimSuffix(line, "\n"))
	if f.fence == "" {
		// Backtick fences can't have backticks in their info string
		if m != nil && !(m[1][0] == '`' && strings.Contains(m[2], "`")) {
			f.fence = m[1]
			return true
		}
		return false
	}
	if m != nil && m[1][0] == f.fence[0] && len(m[1]) >= len(f.fence) && strings.TrimSpace(m[2]) == "" {
		f.fence = ""
	}
	return true
}

// Split splits markdown text on lines of three or more dashes, ignoring any
// that appear inside of fenced code blocks.
func Split(text string) []string {
	var slides, cur []string
	var f fences
	for _, line := range strings.Split(text, "\n") {
		if !f.next(line) && separatorRe.MatchString(line) {
			slides = append(slides, strings.TrimSpace(strings.Join(cur, "\n")))
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	return append(slides, strings.TrimSpace(strings.Join(cur, "\n")))
}

// Steps removes the <!-- pause --> lines (outside of fenced code blocks)
// from a slide, and returns where in the remaining text each one was, so the
// slide can be revealed a step at a time.
func Steps(text string) (string, []int) {
	var sb strings.Builder
	var pauses []int
	var f fences
	for _, line := range strings.SplitAfter(text, "\n") {
		if !f.next(line) && pauseRe.MatchString(line) {
			pauses = append(pauses, sb.Len())
			continue
		}
		sb.WriteString(line)
	}
	text = strings.TrimRight(sb.String(), " \t\n")
	// Pauses at the start or end of the slide don't reveal anything
	pauses = slices.DeleteFunc(pauses, func(p int) bool { return p == 0 || p >= len(text) })
	return text, slices.Compact(pauses)
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
			s, pauses := Steps(s)
			for i, p := range pauses {
				slides = append(slides, Slide{Filename: filename, Text: s, Reveal: p, Step: i})
			}
			slides = append(slides, Slide{Filename: filename, Text: s, Step: len(pauses)})
		}
	default:
		fence := "```"
		for strings.Contains(text, fence) {
			fence += "`"
		}
		slides = append(slides, Slide{Filename: filename, Text: fence + ext + "\n" + strings.TrimSpace(text) + "\n" + fence})
	}
	return slides, nil
}
