package deck

import (
	"slices"
	"testing"
)

func TestSplit(t *testing.T) {
	text := "# One\n\n```\n---\n```\n\n----\n# Two\n~~~~\n---\n~~~\n---\n~~~~\nx\n---\n```` ```demo ````\n---\n# Four"
	want := []string{
		"# One\n\n```\n---\n```",
		"# Two\n~~~~\n---\n~~~\n---\n~~~~\nx",
		"```` ```demo ````",
		"# Four",
	}
	if got := Split(text); !slices.Equal(got, want) {
		t.Errorf("Split() =\n%q\nwant\n%q", got, want)
	}
}
