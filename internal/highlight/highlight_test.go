package highlight

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestBundledLexers(t *testing.T) {
	cases := []struct {
		lang    string
		names   []string
		src     string
		keyword string
		str     string
		comment string
	}{
		{
			lang: "Tomo", names: []string{"tomo", "tm"},
			src:     "# greet\nfunc main()\n    say(\"Hi $name\")\n",
			keyword: "func", str: "Hi ", comment: "# greet",
		},
		{
			lang: "Nomsu", names: []string{"nomsu", "nom"},
			src:     "### say hi\nif (yes):\n    say \"Hello \\$name\"\n",
			keyword: "if", str: "Hello ", comment: "### say hi",
		},
		{
			lang: "BP", names: []string{"bp"},
			src:     "# a rule\nGreeting: \"hello\" __ @who=word\n",
			keyword: "__", str: "\"hello\"", comment: "# a rule",
		},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			for _, name := range tc.names {
				if l := LexerFor(name); l == nil || l.Config().Name != tc.lang {
					t.Fatalf("LexerFor(%q) = %v, want the %s lexer", name, l, tc.lang)
				}
			}
			it, err := chroma.Coalesce(LexerFor(tc.names[0])).Tokenise(nil, tc.src)
			if err != nil {
				t.Fatal(err)
			}
			toks := it.Tokens()
			want := map[string]chroma.TokenType{tc.keyword: chroma.Keyword, tc.str: chroma.LiteralString, tc.comment: chroma.Comment}
			for value, category := range want {
				if typ, ok := typeOf(toks, value); !ok || !typ.InCategory(category) && !typ.InSubCategory(category) {
					t.Errorf("token containing %q has type %v, want %v; tokens: %v", value, typ, category, toks)
				}
			}

			out := Highlight(tc.src, LexerFor(tc.names[0]))
			for i, line := range strings.Split(out, "\n") {
				if strings.Count(line, "\x1b[") != 2*strings.Count(line, "\x1b[m") {
					t.Errorf("line %d has unbalanced styles: %q", i+1, line)
				}
			}
		})
	}
}

// typeOf returns the type of the first token whose value contains s.
func typeOf(toks []chroma.Token, s string) (chroma.TokenType, bool) {
	for _, tok := range toks {
		if strings.Contains(tok.Value, s) {
			return tok.Type, true
		}
	}
	return 0, false
}
