package expression

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer tokenizes an expression string.
type Lexer struct {
	input  string
	pos    int
	tokens []Token
}

// Lex tokenizes the input expression string.
func Lex(input string) ([]Token, error) {
	l := &Lexer{input: input}
	if err := l.lex(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

func (l *Lexer) lex() error {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]

		// Skip whitespace
		if unicode.IsSpace(rune(ch)) {
			l.pos++
			continue
		}

		// String literals
		if ch == '"' || ch == '\'' {
			if err := l.lexString(ch); err != nil {
				return err
			}
			continue
		}

		// Numbers
		if ch >= '0' && ch <= '9' {
			l.lexNumber()
			continue
		}

		// Identifiers and keywords (@ prefix for template vars like @index)
		if ch == '_' || ch == '@' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			l.lexIdent()
			continue
		}

		// Two-character operators
		if l.pos+1 < len(l.input) {
			two := l.input[l.pos : l.pos+2]
			switch two {
			case "==":
				l.emit(TokenEq, two)
				continue
			case "!=":
				l.emit(TokenNeq, two)
				continue
			case ">=":
				l.emit(TokenGte, two)
				continue
			case "<=":
				l.emit(TokenLte, two)
				continue
			case "&&":
				l.emit(TokenAnd, two)
				continue
			case "||":
				l.emit(TokenOr, two)
				continue
			case "??":
				l.emit(TokenNullCoalesce, two)
				continue
			case "?.":
				l.emit(TokenOptionalChain, two)
				continue
			case "=>":
				l.emit(TokenArrow, two)
				continue
			}
		}

		// Single-character operators
		switch ch {
		case '.':
			l.emitSingle(TokenDot)
		case '[':
			l.emitSingle(TokenLBracket)
		case ']':
			l.emitSingle(TokenRBracket)
		case '(':
			l.emitSingle(TokenLParen)
		case ')':
			l.emitSingle(TokenRParen)
		case '{':
			l.emitSingle(TokenLBrace)
		case '}':
			l.emitSingle(TokenRBrace)
		case ',':
			l.emitSingle(TokenComma)
		case ':':
			l.emitSingle(TokenColon)
		case '|':
			l.emitSingle(TokenPipe)
		case '>':
			l.emitSingle(TokenGt)
		case '<':
			l.emitSingle(TokenLt)
		case '!':
			l.emitSingle(TokenNot)
		case '+':
			l.emitSingle(TokenPlus)
		case '-':
			// Check if this is a negative number
			if l.pos+1 < len(l.input) && l.input[l.pos+1] >= '0' && l.input[l.pos+1] <= '9' {
				// Only treat as negative number if preceded by an operator or start of input
				if l.isNegativeNumberContext() {
					l.lexNumber()
					continue
				}
			}
			l.emitSingle(TokenMinus)
		case '*':
			l.emitSingle(TokenStar)
		case '/':
			l.emitSingle(TokenSlash)
		case '%':
			l.emitSingle(TokenPercent)
		case '?':
			l.emitSingle(TokenQuestion)
		default:
			return fmt.Errorf("unexpected character %q at position %d", ch, l.pos)
		}
	}

	l.tokens = append(l.tokens, Token{Type: TokenEOF, Pos: l.pos})
	return nil
}

func (l *Lexer) isNegativeNumberContext() bool {
	if len(l.tokens) == 0 {
		return true
	}
	last := l.tokens[len(l.tokens)-1]
	switch last.Type {
	case TokenLParen, TokenLBracket, TokenComma, TokenColon, TokenPipe, TokenArrow,
		TokenEq, TokenNeq, TokenGt, TokenGte, TokenLt, TokenLte,
		TokenAnd, TokenOr, TokenNot,
		TokenPlus, TokenMinus, TokenStar, TokenSlash, TokenPercent,
		TokenNullCoalesce, TokenQuestion:
		return true
	}
	return false
}

func (l *Lexer) lexString(quote byte) error {
	start := l.pos
	l.pos++ // skip opening quote
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			l.pos++
			switch l.input[l.pos] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '\'':
				sb.WriteByte('\'')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(l.input[l.pos])
			}
			l.pos++
			continue
		}
		if ch == quote {
			l.pos++
			l.tokens = append(l.tokens, Token{Type: TokenString, Value: sb.String(), Pos: start})
			return nil
		}
		sb.WriteByte(ch)
		l.pos++
	}
	return fmt.Errorf("unterminated string starting at position %d", start)
}

func (l *Lexer) lexNumber() {
	start := l.pos
	isFloat := false

	if l.input[l.pos] == '-' {
		l.pos++
	}

	for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
		l.pos++
	}

	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		// Check it's not a method call like 123.toString
		if l.pos+1 < len(l.input) && l.input[l.pos+1] >= '0' && l.input[l.pos+1] <= '9' {
			isFloat = true
			l.pos++
			for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
				l.pos++
			}
		}
	}

	tokType := TokenInt
	if isFloat {
		tokType = TokenFloat
	}
	l.tokens = append(l.tokens, Token{Type: TokenType(tokType), Value: l.input[start:l.pos], Pos: start})
}

func (l *Lexer) lexIdent() {
	start := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '_' || ch == '@' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			l.pos++
		} else {
			break
		}
	}
	word := l.input[start:l.pos]

	switch word {
	case "true", "false":
		l.tokens = append(l.tokens, Token{Type: TokenBool, Value: word, Pos: start})
	case "null":
		l.tokens = append(l.tokens, Token{Type: TokenNull, Value: word, Pos: start})
	default:
		l.tokens = append(l.tokens, Token{Type: TokenIdent, Value: word, Pos: start})
	}
}

func (l *Lexer) emit(typ TokenType, val string) {
	l.tokens = append(l.tokens, Token{Type: typ, Value: val, Pos: l.pos})
	l.pos += len(val)
}

func (l *Lexer) emitSingle(typ TokenType) {
	l.tokens = append(l.tokens, Token{Type: typ, Value: string(l.input[l.pos]), Pos: l.pos})
	l.pos++
}
