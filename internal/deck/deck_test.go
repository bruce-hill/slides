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

func TestSteps(t *testing.T) {
	text := "<!-- pause -->\n# Title\n\n- one\n<!-- pause -->\n- two\n  <!--pause-->  \n\n```\n<!-- pause -->\n```\n<!-- pause -->\n<!-- pause -->"
	got, pauses := Steps(text)
	want := "# Title\n\n- one\n- two\n\n```\n<!-- pause -->\n```"
	if got != want || !slices.Equal(pauses, []int{len("# Title\n\n- one\n"), len("# Title\n\n- one\n- two\n")}) {
		t.Errorf("Steps() = %q, %v", got, pauses)
	}
}
