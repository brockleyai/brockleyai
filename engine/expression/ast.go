package expression

// Expr is the interface for all expression AST nodes.
type Expr interface {
	exprNode()
}

// LiteralExpr represents a literal value (string, int, float, bool, null).
type LiteralExpr struct {
	Value any
}

// IdentExpr represents a variable reference (e.g., "input", "state").
type IdentExpr struct {
	Name string
}

// DotExpr represents field access (e.g., "input.name").
type DotExpr struct {
	Object Expr
	Field  string
}

// IndexExpr represents array index access (e.g., "items[0]").
type IndexExpr struct {
	Object Expr
	Index  Expr
}

// OptionalChainExpr represents ?. access (e.g., "user?.name").
type OptionalChainExpr struct {
	Object Expr
	Field  string
}

// BinaryExpr represents a binary operation (e.g., "a + b", "x == y").
type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

// UnaryExpr represents a unary operation (e.g., "!x", "-y").
type UnaryExpr struct {
	Op      string
	Operand Expr
}

// TernaryExpr represents "condition ? then : else".
type TernaryExpr struct {
	Condition Expr
	Then      Expr
	Else      Expr
}

// NullCoalesceExpr represents "a ?? b".
type NullCoalesceExpr struct {
	Left  Expr
	Right Expr
}

// CallExpr represents a function/method call (e.g., "items.length()", "filter(x => x > 0)").
type CallExpr struct {
	Function Expr
	Args     []Expr
}

// PipeExpr represents "value | filter(args)".
type PipeExpr struct {
	Value  Expr
	Filter string
	Args   []Expr
}

// LambdaExpr represents "x => x.field > 0".
type LambdaExpr struct {
	Param string
	Body  Expr
}

// ArrayExpr represents "[a, b, c]".
type ArrayExpr struct {
	Elements []Expr
}

// ObjectExpr represents "{key: value, ...}".
type ObjectExpr struct {
	Keys   []string
	Values []Expr
}

func (LiteralExpr) exprNode()       {}
func (IdentExpr) exprNode()         {}
func (DotExpr) exprNode()           {}
func (IndexExpr) exprNode()         {}
func (OptionalChainExpr) exprNode() {}
func (BinaryExpr) exprNode()        {}
func (UnaryExpr) exprNode()         {}
func (TernaryExpr) exprNode()       {}
func (NullCoalesceExpr) exprNode()  {}
func (CallExpr) exprNode()          {}
func (PipeExpr) exprNode()          {}
func (LambdaExpr) exprNode()        {}
func (ArrayExpr) exprNode()         {}
func (ObjectExpr) exprNode()        {}
