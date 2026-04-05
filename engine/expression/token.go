// Package expression implements Brockley's expression language for templates,
// conditions, and transforms. See docs/specs/expression-language.md.
package expression

// TokenType identifies the kind of token produced by the lexer.
type TokenType int

const (
	// Literals
	TokenString TokenType = iota
	TokenInt
	TokenFloat
	TokenBool
	TokenNull

	// Identifiers and access
	TokenIdent    // variable name
	TokenDot      // .
	TokenLBracket // [
	TokenRBracket // ]
	TokenLParen   // (
	TokenRParen   // )
	TokenLBrace   // {
	TokenRBrace   // }
	TokenComma    // ,
	TokenColon    // :
	TokenPipe     // |
	TokenArrow    // =>

	// Comparison operators
	TokenEq  // ==
	TokenNeq // !=
	TokenGt  // >
	TokenGte // >=
	TokenLt  // <
	TokenLte // <=

	// Logical operators
	TokenAnd // &&
	TokenOr  // ||
	TokenNot // !

	// Arithmetic operators
	TokenPlus    // +
	TokenMinus   // -
	TokenStar    // *
	TokenSlash   // /
	TokenPercent // %

	// Null handling
	TokenNullCoalesce  // ??
	TokenOptionalChain // ?.

	// Ternary
	TokenQuestion // ?

	// Special
	TokenEOF
)

// Token is a lexical unit produced by the lexer.
type Token struct {
	Type  TokenType
	Value string
	Pos   int
}
