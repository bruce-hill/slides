// Package highlight syntax highlights code for display in a terminal.
package highlight

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Lexers for languages that chroma doesn't have built in
//
//go:embed lexers/*.xml
var bundled embed.FS

func init() {
	sub, _ := fs.Sub(bundled, "lexers")
	files, _ := fs.Glob(sub, "*.xml")
	for _, f := range files {
		lexers.Register(chroma.MustNewXMLLexer(sub, f))
	}
}

var theme = func() *chroma.Style {
	style, err := styles.Get("vim").Builder().
		Add(chroma.Comment, "italic #59c3a5").
		Add(chroma.LiteralString, "#ed8cb3").
		Add(chroma.LiteralStringInterpol, "bold #96d2ff").
		Add(chroma.Operator, "#febf1a").
		Add(chroma.Keyword, "#febf1a").
		Add(chroma.KeywordDeclaration, "#febf1a").
		Add(chroma.KeywordType, "#61ddb9").
		Add(chroma.Name, "#fafad0").
		Add(chroma.NameVariableMagic, "bold #ffffff").
		Add(chroma.GenericPrompt, "bold #96d2ff").
		Add(chroma.GenericOutput, "#a0a0a0").
		Build()
	if err != nil {
		panic(err)
	}
	return style
}()

// LexerFor finds a lexer by language name or file extension, or nil.
func LexerFor(name string) chroma.Lexer {
	if name == "" {
		return nil
	}
	if l := lexers.Get(name); l != nil {
		return l
	}
	return lexers.Match("file." + name)
}

func sgr(e chroma.StyleEntry) string {
	var codes []string
	if e.Bold == chroma.Yes {
		codes = append(codes, "1")
	}
	if e.Italic == chroma.Yes {
		codes = append(codes, "3")
	}
	if e.Underline == chroma.Yes {
		codes = append(codes, "4")
	}
	if e.Colour.IsSet() && e.Colour != theme.Get(chroma.Background).Colour {
		codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", e.Colour.Red(), e.Colour.Green(), e.Colour.Blue()))
	}
	if len(codes) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

type cacheKey struct{ lexer, code string }

// Slides are redrawn many times a second while a demo's terminal is
// changing, so the code on them is only highlighted once.
var (
	cacheMu sync.Mutex
	cache   = map[cacheKey]string{}
)

// Highlight syntax highlights code. Styles never span across lines, so each
// line of output can be displayed independently.
func Highlight(code string, lexer chroma.Lexer) string {
	code = strings.Trim(strings.ReplaceAll(code, "\t", "    "), "\n")
	if lexer == nil {
		return code
	}
	key := cacheKey{lexer.Config().Name, code}
	cacheMu.Lock()
	out, ok := cache[key]
	cacheMu.Unlock()
	if !ok {
		out = highlight(code, lexer)
		cacheMu.Lock()
		cache[key] = out
		cacheMu.Unlock()
	}
	return out
}

func highlight(code string, lexer chroma.Lexer) string {
	it, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		return code
	}
	var sb strings.Builder
	for _, tok := range it.Tokens() {
		on := sgr(theme.Get(tok.Type))
		if on == "" {
			sb.WriteString(tok.Value)
			continue
		}
		for i, part := range strings.Split(tok.Value, "\n") {
			if i > 0 {
				sb.WriteByte('\n')
			}
			if part != "" {
				sb.WriteString(on + part + "\x1b[m")
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
