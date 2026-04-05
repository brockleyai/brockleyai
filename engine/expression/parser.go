package expression

import (
	"fmt"
	"strconv"
)

// parser is a recursive-descent parser that converts tokens into an AST.
type parser struct {
	tokens []Token
	pos    int
}

// Parse lexes the input string and parses it into an AST node.
func Parse(input string) (Expr, error) {
	tokens, err := Lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.peek().Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token %q at position %d", p.peek().Value, p.peek().Pos)
	}
	return expr, nil
}

func (p *parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(typ TokenType) (Token, error) {
	tok := p.peek()
	if tok.Type != typ {
		return tok, fmt.Errorf("expected token type %d but got %q at position %d", typ, tok.Value, tok.Pos)
	}
	return p.advance(), nil
}

// parseExpression is the entry point: handles pipe, then ternary, etc.
func (p *parser) parseExpression() (Expr, error) {
	return p.parsePipe()
}

// parsePipe handles: expr | filter(args) | filter2
func (p *parser) parsePipe() (Expr, error) {
	expr, err := p.parseTernary()
	if err != nil {
		return nil, err
	}

	for p.peek().Type == TokenPipe {
		p.advance() // consume |
		nameTok, err := p.expect(TokenIdent)
		if err != nil {
			return nil, fmt.Errorf("expected filter name after |: %w", err)
		}
		var args []Expr
		if p.peek().Type == TokenLParen {
			p.advance() // consume (
			if p.peek().Type != TokenRParen {
				args, err = p.parseArgList()
				if err != nil {
					return nil, err
				}
			}
			if _, err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
		}
		expr = &PipeExpr{Value: expr, Filter: nameTok.Value, Args: args}
	}
	return expr, nil
}

// parseTernary handles: expr ? expr : expr
func (p *parser) parseTernary() (Expr, error) {
	expr, err := p.parseNullCoalesce()
	if err != nil {
		return nil, err
	}

	if p.peek().Type == TokenQuestion {
		p.advance() // consume ?
		then, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, fmt.Errorf("expected ':' in ternary expression: %w", err)
		}
		els, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		return &TernaryExpr{Condition: expr, Then: then, Else: els}, nil
	}
	return expr, nil
}

// parseNullCoalesce handles: expr ?? expr
func (p *parser) parseNullCoalesce() (Expr, error) {
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	for p.peek().Type == TokenNullCoalesce {
		p.advance()
		right, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		expr = &NullCoalesceExpr{Left: expr, Right: right}
	}
	return expr, nil
}

// parseOr handles: expr || expr
func (p *parser) parseOr() (Expr, error) {
	expr, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		expr = &BinaryExpr{Op: "||", Left: expr, Right: right}
	}
	return expr, nil
}

// parseAnd handles: expr && expr
func (p *parser) parseAnd() (Expr, error) {
	expr, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenAnd {
		p.advance()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		expr = &BinaryExpr{Op: "&&", Left: expr, Right: right}
	}
	return expr, nil
}

// parseEquality handles: == !=
func (p *parser) parseEquality() (Expr, error) {
	expr, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenEq || p.peek().Type == TokenNeq {
		tok := p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		expr = &BinaryExpr{Op: tok.Value, Left: expr, Right: right}
	}
	return expr, nil
}

// parseComparison handles: > >= < <=
func (p *parser) parseComparison() (Expr, error) {
	expr, err := p.parseAddition()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenGt || p.peek().Type == TokenGte ||
		p.peek().Type == TokenLt || p.peek().Type == TokenLte {
		tok := p.advance()
		right, err := p.parseAddition()
		if err != nil {
			return nil, err
		}
		expr = &BinaryExpr{Op: tok.Value, Left: expr, Right: right}
	}
	return expr, nil
}

// parseAddition handles: + -
func (p *parser) parseAddition() (Expr, error) {
	expr, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenPlus || p.peek().Type == TokenMinus {
		tok := p.advance()
		right, err := p.parseMultiplication()
		if err != nil {
			return nil, err
		}
		expr = &BinaryExpr{Op: tok.Value, Left: expr, Right: right}
	}
	return expr, nil
}

// parseMultiplication handles: * / %
func (p *parser) parseMultiplication() (Expr, error) {
	expr, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenStar || p.peek().Type == TokenSlash || p.peek().Type == TokenPercent {
		tok := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		expr = &BinaryExpr{Op: tok.Value, Left: expr, Right: right}
	}
	return expr, nil
}

// parseUnary handles: !expr
func (p *parser) parseUnary() (Expr, error) {
	if p.peek().Type == TokenNot {
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "!", Operand: operand}, nil
	}
	return p.parsePostfix()
}

// parsePostfix handles: . ?. [] () chains
func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		switch p.peek().Type {
		case TokenDot:
			p.advance()
			nameTok, err := p.expect(TokenIdent)
			if err != nil {
				return nil, fmt.Errorf("expected field name after '.': %w", err)
			}
			// Check if this is a method call: ident(args)
			if p.peek().Type == TokenLParen {
				p.advance() // consume (
				var args []Expr
				if p.peek().Type != TokenRParen {
					args, err = p.parseArgList()
					if err != nil {
						return nil, err
					}
				}
				if _, err := p.expect(TokenRParen); err != nil {
					return nil, err
				}
				// Treat as pipe: value.filter(args) => PipeExpr
				expr = &PipeExpr{Value: expr, Filter: nameTok.Value, Args: args}
			} else {
				expr = &DotExpr{Object: expr, Field: nameTok.Value}
			}

		case TokenOptionalChain:
			p.advance()
			nameTok, err := p.expect(TokenIdent)
			if err != nil {
				return nil, fmt.Errorf("expected field name after '?.': %w", err)
			}
			expr = &OptionalChainExpr{Object: expr, Field: nameTok.Value}

		case TokenLBracket:
			p.advance()
			index, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokenRBracket); err != nil {
				return nil, err
			}
			expr = &IndexExpr{Object: expr, Index: index}

		case TokenLParen:
			// Direct function call on an expression (not dot-access)
			p.advance()
			var args []Expr
			if p.peek().Type != TokenRParen {
				args, err = p.parseArgList()
				if err != nil {
					return nil, err
				}
			}
			if _, err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
			expr = &CallExpr{Function: expr, Args: args}

		default:
			return expr, nil
		}
	}
}

// parsePrimary handles: literals, identifiers (with possible lambda), parens, arrays, objects
func (p *parser) parsePrimary() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenString:
		p.advance()
		return &LiteralExpr{Value: tok.Value}, nil

	case TokenInt:
		p.advance()
		v, err := strconv.ParseInt(tok.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", tok.Value, err)
		}
		return &LiteralExpr{Value: v}, nil

	case TokenFloat:
		p.advance()
		v, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", tok.Value, err)
		}
		return &LiteralExpr{Value: v}, nil

	case TokenBool:
		p.advance()
		return &LiteralExpr{Value: tok.Value == "true"}, nil

	case TokenNull:
		p.advance()
		return &LiteralExpr{Value: nil}, nil

	case TokenIdent:
		p.advance()
		// Check for lambda: ident => body
		if p.peek().Type == TokenArrow {
			p.advance() // consume =>
			body, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			return &LambdaExpr{Param: tok.Value, Body: body}, nil
		}
		return &IdentExpr{Name: tok.Value}, nil

	case TokenLParen:
		p.advance() // consume (
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil

	case TokenLBracket:
		return p.parseArrayLiteral()

	case TokenLBrace:
		return p.parseObjectLiteral()

	default:
		return nil, fmt.Errorf("unexpected token %q at position %d", tok.Value, tok.Pos)
	}
}

func (p *parser) parseArrayLiteral() (Expr, error) {
	p.advance() // consume [
	var elements []Expr
	if p.peek().Type != TokenRBracket {
		for {
			elem, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)
			if p.peek().Type != TokenComma {
				break
			}
			p.advance() // consume ,
		}
	}
	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return &ArrayExpr{Elements: elements}, nil
}

func (p *parser) parseObjectLiteral() (Expr, error) {
	p.advance() // consume {
	var keys []string
	var values []Expr
	if p.peek().Type != TokenRBrace {
		for {
			keyTok := p.peek()
			if keyTok.Type != TokenIdent && keyTok.Type != TokenString {
				return nil, fmt.Errorf("expected object key at position %d", keyTok.Pos)
			}
			p.advance()
			if _, err := p.expect(TokenColon); err != nil {
				return nil, err
			}
			val, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			keys = append(keys, keyTok.Value)
			values = append(values, val)
			if p.peek().Type != TokenComma {
				break
			}
			p.advance() // consume ,
		}
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return &ObjectExpr{Keys: keys, Values: values}, nil
}

func (p *parser) parseArgList() ([]Expr, error) {
	var args []Expr
	for {
		arg, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.peek().Type != TokenComma {
			break
		}
		p.advance() // consume ,
	}
	return args, nil
}
