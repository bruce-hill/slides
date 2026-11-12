package highlight

// Chroma lexer for the Tomo programming language.
// Based on the Tomo vim syntax file by Bruce Hill <bruce@bruce-hill.com>

import (
	. "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

var _ = lexers.Register(MustNewLexer(
	&Config{Name: "Tomo", Aliases: []string{"tomo"}, Filenames: []string{"*.tm"}},
	tomoRules,
))

func rule(pattern string, emitter Emitter, mutator Mutator) Rule {
	return Rule{Pattern: pattern, Type: emitter, Mutator: mutator}
}

func tomoRules() Rules {
	const escape = `\\(?:[abenrtvN]|x[0-9a-fA-F]{2}|\d{3}|\{[^}]*\}|\[[^\]]*\]|.)`
	interpolated := func(end string, tok TokenType) []Rule {
		return []Rule{
			rule(end, tok, Pop(1)),
			rule(escape, LiteralStringEscape, nil),
			rule(`\$[A-Za-z_]\w*`, LiteralStringInterpol, nil),
			rule(`\$\(`, LiteralStringInterpol, Push("interp_paren")),
			rule(`\$`, LiteralStringInterpol, nil),
			rule(`[^`+end+`\\$]+`, tok, nil),
		}
	}
	return Rules{
		"root": {
			rule(`^\s*!link.*$`, CommentPreproc, nil),
			rule(`^\s*>>>`, GenericPrompt, nil),
			rule(`^\s*===.*$`, GenericPrompt, nil),
			rule(`!!!.*$`, GenericError, nil),
			rule(`#.*$`, CommentSingle, nil),
			rule(`!!`, GenericStrong, Push("say_line")),
			rule(`\buse\b`, Keyword, Push("use_line")),
			rule(`\bC_code\b`, Keyword, nil),
			rule(`\$\d*[A-Za-z_]\w*`, NameVariableMagic, nil),
			rule(Words(``, `\b`, "use", "if", "unless", "else", "when", "then", "defer", "for", "while",
				"do", "until", "repeat"), Keyword, nil),
			rule(Words(``, `\b`, "stop", "skip", "break", "continue", "pass", "return", "assert"), Keyword, nil),
			rule(Words(``, `\b`, "func", "lang", "struct", "convert", "enum"), KeywordDeclaration, nil),
			rule(Words(``, `\b`, "deserialize"), Keyword, nil),
			rule(Words(``, `\b`, "in", "and", "or", "xor", "is", "not", "mod", "mod1",
				"_min_", "_max_", "_mix_", "mutexed"), OperatorWord, nil),
			rule(Words(``, `\b`, "yes", "no", "none"), KeywordConstant, nil),
			rule(`\((?:~/|\.\.?/|/)(?:[^)\\]|\\.)*\)`, LiteralString, nil),
			rule("`.", LiteralStringChar, nil),
			rule(`"`, LiteralStringDouble, Push("dq_string")),
			rule(`'`, LiteralStringSingle, Push("sq_string")),
			rule("`", LiteralStringBacktick, Push("bt_string")),
			rule(`0x[0-9a-fA-F_]+%?`, LiteralNumberHex, nil),
			rule(`[0-9][0-9_]*(?:\.(?:[0-9][0-9_]*))?(?:e[0-9_]+)?%?`, LiteralNumber, nil),
			rule(`\.[0-9][0-9_]*(?:e[0-9_]+)?%?`, LiteralNumberFloat, nil),
			rule(`->|[:!]?=|[+*^<>=-]=?|[#?:!@&]|>>`, Operator, nil),
			rule(`[,(){}\[\];.]`, Punctuation, nil),
			rule(Words(``, `\b`, "Int", "Bool", "Num", "Text", "Void", "Abort",
				"Int64", "Int32", "Int16", "Int8", "Byte"), KeywordType, nil),
			rule(`[A-Za-z_]\w*`, Name, nil),
			rule(`\s+`, TextWhitespace, nil),
		},
		"dq_string": interpolated(`"`, LiteralStringDouble),
		"sq_string": interpolated(`'`, LiteralStringSingle),
		"bt_string": interpolated("`", LiteralStringBacktick),
		"interp_paren": {
			rule(`\)`, LiteralStringInterpol, Pop(1)),
			Include("root"),
		},
		"say_line": {
			rule(`$`, Text, Pop(1)),
			rule(escape, LiteralStringEscape, nil),
			rule(`\$[A-Za-z_]\w*`, LiteralStringInterpol, nil),
			rule(`\$\(`, LiteralStringInterpol, Push("interp_paren")),
			rule(`[^\\$\n]+`, GenericStrong, nil),
		},
		"use_line": {
			rule(`$|;`, Text, Pop(1)),
			rule(`[^\n;]+`, LiteralString, nil),
		},
	}
}
