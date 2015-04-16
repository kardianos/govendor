package kingpin

import (
	"bufio"
	"os"

	"strings"
)

type TokenType int

// Token types.
const (
	TokenShort TokenType = iota
	TokenLong
	TokenArg
	TokenEOL
)

var (
	TokenEOLMarker = Token{TokenEOL, ""}
)

type Token struct {
	Type  TokenType
	Value string
}

func (t *Token) IsFlag() bool {
	return t.Type == TokenShort || t.Type == TokenLong
}

func (t *Token) IsEOF() bool {
	return t.Type == TokenEOL
}

func (t *Token) String() string {
	switch t.Type {
	case TokenShort:
		return "-" + t.Value
	case TokenLong:
		return "--" + t.Value
	case TokenArg:
		return t.Value
	case TokenEOL:
		return "<EOL>"
	default:
		panic("unhandled type")
	}
}

type Tokens []*Token

func (t Tokens) String() string {
	out := []string{}
	for _, tok := range t {
		out = append(out, tok.String())
	}
	return strings.Join(out, " ")
}

func (t Tokens) Next() Tokens {
	if len(t) == 0 {
		return nil
	}
	return t[1:]
}

func (t Tokens) Return(token *Token) Tokens {
	if token.Type == TokenEOL {
		return t
	}
	return append(Tokens{token}, t...)
}

func (t Tokens) Peek() *Token {
	if len(t) == 0 {
		return &TokenEOLMarker
	}
	return t[0]
}

func Tokenize(args []string) *ParseContext {
	tokens := make(Tokens, 0, len(args))
	allowFlags := true
	for _, arg := range args {
		if allowFlags {
			if arg == "--" {
				allowFlags = false
				continue
			}
			if strings.HasPrefix(arg, "--") {
				parts := strings.SplitN(arg[2:], "=", 2)
				tokens = append(tokens, &Token{TokenLong, parts[0]})
				if len(parts) == 2 {
					tokens = append(tokens, &Token{TokenArg, parts[1]})
				}
				continue
			}
			if strings.HasPrefix(arg, "-") {
				for _, a := range arg[1:] {
					tokens = append(tokens, &Token{TokenShort, string(a)})
				}
				continue
			}
		}
		tokens = append(tokens, &Token{TokenArg, arg})
	}
	return &ParseContext{Tokens: tokens}
}

// ExpandArgsFromFiles expands arguments in the form @<file> into one-arg-per-
// line read from that file.
func ExpandArgsFromFiles(args []string) ([]string, error) {
	out := []string{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			r, err := os.Open(arg[1:])
			if err != nil {
				return nil, err
			}
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				out = append(out, scanner.Text())
			}
			r.Close()
			if scanner.Err() != nil {
				return nil, scanner.Err()
			}
		} else {
			out = append(out, arg)
		}
	}
	return out, nil
}
