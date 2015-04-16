package kingpin

type ParseContext struct {
	Tokens          Tokens
	SelectedCommand string
}

func (p *ParseContext) Next() {
	p.Tokens = p.Tokens.Next()
}

func (p *ParseContext) Peek() *Token {
	return p.Tokens.Peek()
}

func (p *ParseContext) Return(token *Token) {
	p.Tokens = p.Tokens.Return(token)
}

func (p *ParseContext) String() string {
	return p.SelectedCommand + ": " + p.Tokens.String()
}
