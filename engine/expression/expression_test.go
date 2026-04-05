package expression

import (
	"reflect"
	"testing"
)

// --- Lexer tests ---

func TestLex(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []TokenType
	}{
		{
			name:   "simple identifier",
			input:  "input",
			tokens: []TokenType{TokenIdent, TokenEOF},
		},
		{
			name:   "dotted access",
			input:  "input.name",
			tokens: []TokenType{TokenIdent, TokenDot, TokenIdent, TokenEOF},
		},
		{
			name:   "string literal double quotes",
			input:  `"hello world"`,
			tokens: []TokenType{TokenString, TokenEOF},
		},
		{
			name:   "string literal single quotes",
			input:  `'hello'`,
			tokens: []TokenType{TokenString, TokenEOF},
		},
		{
			name:   "integer",
			input:  "42",
			tokens: []TokenType{TokenInt, TokenEOF},
		},
		{
			name:   "float",
			input:  "3.14",
			tokens: []TokenType{TokenFloat, TokenEOF},
		},
		{
			name:   "negative integer",
			input:  "-5",
			tokens: []TokenType{TokenInt, TokenEOF},
		},
		{
			name:   "boolean true",
			input:  "true",
			tokens: []TokenType{TokenBool, TokenEOF},
		},
		{
			name:   "boolean false",
			input:  "false",
			tokens: []TokenType{TokenBool, TokenEOF},
		},
		{
			name:   "null",
			input:  "null",
			tokens: []TokenType{TokenNull, TokenEOF},
		},
		{
			name:   "comparison operators",
			input:  "a == b != c > d >= e < f <= g",
			tokens: []TokenType{TokenIdent, TokenEq, TokenIdent, TokenNeq, TokenIdent, TokenGt, TokenIdent, TokenGte, TokenIdent, TokenLt, TokenIdent, TokenLte, TokenIdent, TokenEOF},
		},
		{
			name:   "logical operators",
			input:  "a && b || !c",
			tokens: []TokenType{TokenIdent, TokenAnd, TokenIdent, TokenOr, TokenNot, TokenIdent, TokenEOF},
		},
		{
			name:   "arithmetic operators",
			input:  "a + b - c * d / e % f",
			tokens: []TokenType{TokenIdent, TokenPlus, TokenIdent, TokenMinus, TokenIdent, TokenStar, TokenIdent, TokenSlash, TokenIdent, TokenPercent, TokenIdent, TokenEOF},
		},
		{
			name:   "null coalesce and optional chain",
			input:  "a ?? b?.c",
			tokens: []TokenType{TokenIdent, TokenNullCoalesce, TokenIdent, TokenOptionalChain, TokenIdent, TokenEOF},
		},
		{
			name:   "ternary",
			input:  "a ? b : c",
			tokens: []TokenType{TokenIdent, TokenQuestion, TokenIdent, TokenColon, TokenIdent, TokenEOF},
		},
		{
			name:   "arrow",
			input:  "x => x + 1",
			tokens: []TokenType{TokenIdent, TokenArrow, TokenIdent, TokenPlus, TokenInt, TokenEOF},
		},
		{
			name:   "brackets and parens",
			input:  "items[0].filter(x => x)",
			tokens: []TokenType{TokenIdent, TokenLBracket, TokenInt, TokenRBracket, TokenDot, TokenIdent, TokenLParen, TokenIdent, TokenArrow, TokenIdent, TokenRParen, TokenEOF},
		},
		{
			name:   "pipe",
			input:  "value | upper",
			tokens: []TokenType{TokenIdent, TokenPipe, TokenIdent, TokenEOF},
		},
		{
			name:   "object literal",
			input:  `{a: 1, b: "two"}`,
			tokens: []TokenType{TokenLBrace, TokenIdent, TokenColon, TokenInt, TokenComma, TokenIdent, TokenColon, TokenString, TokenRBrace, TokenEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Lex(tt.input)
			if err != nil {
				t.Fatalf("Lex(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.tokens) {
				t.Fatalf("Lex(%q) got %d tokens, want %d\ntokens: %+v", tt.input, len(tokens), len(tt.tokens), tokens)
			}
			for i, tok := range tokens {
				if tok.Type != tt.tokens[i] {
					t.Errorf("token[%d] type = %d, want %d (value=%q)", i, tok.Type, tt.tokens[i], tok.Value)
				}
			}
		})
	}
}

// --- Parser tests ---

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, expr Expr)
	}{
		{
			name:  "literal int",
			input: "42",
			check: func(t *testing.T, expr Expr) {
				lit, ok := expr.(*LiteralExpr)
				if !ok {
					t.Fatalf("expected LiteralExpr, got %T", expr)
				}
				if lit.Value != int64(42) {
					t.Errorf("value = %v, want 42", lit.Value)
				}
			},
		},
		{
			name:  "literal string",
			input: `"hello"`,
			check: func(t *testing.T, expr Expr) {
				lit, ok := expr.(*LiteralExpr)
				if !ok {
					t.Fatalf("expected LiteralExpr, got %T", expr)
				}
				if lit.Value != "hello" {
					t.Errorf("value = %v, want hello", lit.Value)
				}
			},
		},
		{
			name:  "identifier",
			input: "input",
			check: func(t *testing.T, expr Expr) {
				id, ok := expr.(*IdentExpr)
				if !ok {
					t.Fatalf("expected IdentExpr, got %T", expr)
				}
				if id.Name != "input" {
					t.Errorf("name = %q, want input", id.Name)
				}
			},
		},
		{
			name:  "dot access",
			input: "input.user.name",
			check: func(t *testing.T, expr Expr) {
				dot, ok := expr.(*DotExpr)
				if !ok {
					t.Fatalf("expected DotExpr, got %T", expr)
				}
				if dot.Field != "name" {
					t.Errorf("field = %q, want name", dot.Field)
				}
				inner, ok := dot.Object.(*DotExpr)
				if !ok {
					t.Fatalf("expected inner DotExpr, got %T", dot.Object)
				}
				if inner.Field != "user" {
					t.Errorf("inner field = %q, want user", inner.Field)
				}
			},
		},
		{
			name:  "optional chain",
			input: "input?.user",
			check: func(t *testing.T, expr Expr) {
				oc, ok := expr.(*OptionalChainExpr)
				if !ok {
					t.Fatalf("expected OptionalChainExpr, got %T", expr)
				}
				if oc.Field != "user" {
					t.Errorf("field = %q, want user", oc.Field)
				}
			},
		},
		{
			name:  "index access",
			input: "items[0]",
			check: func(t *testing.T, expr Expr) {
				idx, ok := expr.(*IndexExpr)
				if !ok {
					t.Fatalf("expected IndexExpr, got %T", expr)
				}
				_ = idx
			},
		},
		{
			name:  "binary expression",
			input: "a + b * c",
			check: func(t *testing.T, expr Expr) {
				bin, ok := expr.(*BinaryExpr)
				if !ok {
					t.Fatalf("expected BinaryExpr, got %T", expr)
				}
				if bin.Op != "+" {
					t.Errorf("op = %q, want +", bin.Op)
				}
				// Right should be * (higher precedence)
				right, ok := bin.Right.(*BinaryExpr)
				if !ok {
					t.Fatalf("expected BinaryExpr on right, got %T", bin.Right)
				}
				if right.Op != "*" {
					t.Errorf("right op = %q, want *", right.Op)
				}
			},
		},
		{
			name:  "unary not",
			input: "!active",
			check: func(t *testing.T, expr Expr) {
				un, ok := expr.(*UnaryExpr)
				if !ok {
					t.Fatalf("expected UnaryExpr, got %T", expr)
				}
				if un.Op != "!" {
					t.Errorf("op = %q, want !", un.Op)
				}
			},
		},
		{
			name:  "ternary",
			input: "x > 0 ? x : 0",
			check: func(t *testing.T, expr Expr) {
				tern, ok := expr.(*TernaryExpr)
				if !ok {
					t.Fatalf("expected TernaryExpr, got %T", expr)
				}
				_ = tern
			},
		},
		{
			name:  "null coalesce",
			input: "a ?? b",
			check: func(t *testing.T, expr Expr) {
				nc, ok := expr.(*NullCoalesceExpr)
				if !ok {
					t.Fatalf("expected NullCoalesceExpr, got %T", expr)
				}
				_ = nc
			},
		},
		{
			name:  "pipe expression",
			input: "items | length",
			check: func(t *testing.T, expr Expr) {
				pipe, ok := expr.(*PipeExpr)
				if !ok {
					t.Fatalf("expected PipeExpr, got %T", expr)
				}
				if pipe.Filter != "length" {
					t.Errorf("filter = %q, want length", pipe.Filter)
				}
			},
		},
		{
			name:  "pipe with args",
			input: `items | join(", ")`,
			check: func(t *testing.T, expr Expr) {
				pipe, ok := expr.(*PipeExpr)
				if !ok {
					t.Fatalf("expected PipeExpr, got %T", expr)
				}
				if pipe.Filter != "join" {
					t.Errorf("filter = %q, want join", pipe.Filter)
				}
				if len(pipe.Args) != 1 {
					t.Errorf("args len = %d, want 1", len(pipe.Args))
				}
			},
		},
		{
			name:  "lambda",
			input: "x => x + 1",
			check: func(t *testing.T, expr Expr) {
				lam, ok := expr.(*LambdaExpr)
				if !ok {
					t.Fatalf("expected LambdaExpr, got %T", expr)
				}
				if lam.Param != "x" {
					t.Errorf("param = %q, want x", lam.Param)
				}
			},
		},
		{
			name:  "array literal",
			input: "[1, 2, 3]",
			check: func(t *testing.T, expr Expr) {
				arr, ok := expr.(*ArrayExpr)
				if !ok {
					t.Fatalf("expected ArrayExpr, got %T", expr)
				}
				if len(arr.Elements) != 3 {
					t.Errorf("elements len = %d, want 3", len(arr.Elements))
				}
			},
		},
		{
			name:  "object literal",
			input: `{name: "Alice", age: 30}`,
			check: func(t *testing.T, expr Expr) {
				obj, ok := expr.(*ObjectExpr)
				if !ok {
					t.Fatalf("expected ObjectExpr, got %T", expr)
				}
				if len(obj.Keys) != 2 {
					t.Errorf("keys len = %d, want 2", len(obj.Keys))
				}
			},
		},
		{
			name:  "method call becomes pipe",
			input: "items.length()",
			check: func(t *testing.T, expr Expr) {
				pipe, ok := expr.(*PipeExpr)
				if !ok {
					t.Fatalf("expected PipeExpr, got %T", expr)
				}
				if pipe.Filter != "length" {
					t.Errorf("filter = %q, want length", pipe.Filter)
				}
			},
		},
		{
			name:  "parenthesized expression",
			input: "(a + b) * c",
			check: func(t *testing.T, expr Expr) {
				bin, ok := expr.(*BinaryExpr)
				if !ok {
					t.Fatalf("expected BinaryExpr, got %T", expr)
				}
				if bin.Op != "*" {
					t.Errorf("op = %q, want *", bin.Op)
				}
				// Left should be + (inside parens)
				left, ok := bin.Left.(*BinaryExpr)
				if !ok {
					t.Fatalf("expected BinaryExpr on left, got %T", bin.Left)
				}
				if left.Op != "+" {
					t.Errorf("left op = %q, want +", left.Op)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			tt.check(t, expr)
		})
	}
}

// --- Eval tests ---

func TestEval(t *testing.T) {
	ctx := &Context{
		Input: map[string]any{
			"name": "Alice",
			"age":  int64(30),
			"user": map[string]any{
				"name": "Bob",
				"address": map[string]any{
					"city": "London",
				},
			},
			"items":   []any{int64(1), int64(2), int64(3), int64(4), int64(5)},
			"tags":    []any{"go", "rust", "python"},
			"score":   3.14,
			"active":  true,
			"nothing": nil,
			"objects": []any{
				map[string]any{"name": "Alice", "score": int64(90)},
				map[string]any{"name": "Bob", "score": int64(80)},
			},
			"nested": []any{
				[]any{int64(1), int64(2)},
				[]any{int64(3), int64(4)},
			},
			"greeting": "Hello, World!",
		},
		State: map[string]any{
			"counter": int64(10),
		},
		Meta: map[string]any{
			"version": "1.0",
		},
	}

	tests := []struct {
		name string
		expr string
		want any
	}{
		// Literals
		{"int literal", "42", int64(42)},
		{"float literal", "3.14", 3.14},
		{"string literal", `"hello"`, "hello"},
		{"bool literal true", "true", true},
		{"bool literal false", "false", false},
		{"null literal", "null", nil},

		// Field access
		{"simple field", "input.name", "Alice"},
		{"nested field", "input.user.name", "Bob"},
		{"deep nested field", "input.user.address.city", "London"},
		{"state access", "state.counter", int64(10)},
		{"meta access", "meta.version", "1.0"},

		// Missing field returns nil
		{"missing field", "input.nonexistent", nil},
		{"missing deep field", "input.user.missing.deep", nil},

		// Optional chaining
		{"optional chain present", "input.user?.name", "Bob"},
		{"optional chain nil", "input.nothing?.name", nil},

		// Index access
		{"positive index", "input.items[0]", int64(1)},
		{"negative index", "input.items[-1]", int64(5)},
		{"out of bounds index", "input.items[10]", nil},

		// Comparison operators
		{"equal true", "1 == 1", true},
		{"equal false", "1 == 2", false},
		{"not equal", "1 != 2", true},
		{"greater than true", "2 > 1", true},
		{"greater than false", "1 > 2", false},
		{"greater equal", "2 >= 2", true},
		{"less than", "1 < 2", true},
		{"less equal", "2 <= 2", true},
		{"string equality", `"abc" == "abc"`, true},

		// Arithmetic int
		{"add ints", "2 + 3", int64(5)},
		{"subtract ints", "10 - 4", int64(6)},
		{"multiply ints", "3 * 4", int64(12)},
		{"divide ints", "10 / 2", int64(5)},
		{"modulo", "10 % 3", int64(1)},

		// Arithmetic float
		{"add float", "1.5 + 2.5", 4.0},
		{"multiply float", "2.0 * 3.0", 6.0},

		// String concatenation
		{"string concat", `"hello" + " " + "world"`, "hello world"},

		// Logical operators with short-circuit
		{"and true", "true && true", true},
		{"and false", "true && false", false},
		{"and short-circuit", "false && true", false},
		{"or true", "false || true", true},
		{"or short-circuit", "true || false", true},
		{"not true", "!true", false},
		{"not false", "!false", true},

		// Null coalescing
		{"null coalesce present", `input.name ?? "default"`, "Alice"},
		{"null coalesce nil", `input.nothing ?? "default"`, "default"},
		{"null coalesce chain", `input.nothing ?? input.missing ?? "fallback"`, "fallback"},

		// Ternary
		{"ternary true", `true ? "yes" : "no"`, "yes"},
		{"ternary false", `false ? "yes" : "no"`, "no"},
		{"ternary expr", `input.age > 18 ? "adult" : "minor"`, "adult"},

		// Division by zero
		{"division by zero int", "10 / 0", nil},
		{"modulo by zero", "10 % 0", nil},

		// Array literal
		{"array literal", "[1, 2, 3]", []any{int64(1), int64(2), int64(3)}},
		{"empty array", "[]", []any{}},

		// Object literal
		{"object literal", `{name: "test"}`, map[string]any{"name": "test"}},

		// Pipe filters: length
		{"length array", "input.items | length", int64(5)},
		{"length string", "input.name | length", int64(5)},

		// Pipe filters: first, last
		{"first", "input.items | first", int64(1)},
		{"last", "input.items | last", int64(5)},

		// Pipe filters: join
		{"join", `input.tags | join(", ")`, "go, rust, python"},

		// Pipe filters: contains
		{"contains array true", `input.tags | contains("go")`, true},
		{"contains array false", `input.tags | contains("java")`, false},
		{"contains string", `input.greeting | contains("World")`, true},

		// Pipe filters: map
		{"map field", `input.objects | map("name")`, []any{"Alice", "Bob"}},

		// Pipe filters: filter with lambda
		{"filter lambda", "input.items | filter(x => x > 3)", []any{int64(4), int64(5)}},

		// Pipe filters: sum
		{"sum", "input.items | sum", int64(15)},

		// Pipe filters: sort
		{"sort", "[3, 1, 2] | sort", []any{int64(1), int64(2), int64(3)}},

		// Pipe filters: reverse
		{"reverse", "[1, 2, 3] | reverse", []any{int64(3), int64(2), int64(1)}},

		// Pipe filters: flatten
		{"flatten", "input.nested | flatten", []any{int64(1), int64(2), int64(3), int64(4)}},

		// Pipe filters: unique
		{"unique", "[1, 2, 2, 3, 1] | unique", []any{int64(1), int64(2), int64(3)}},

		// Pipe filters: slice
		{"slice", "input.items | slice(1, 3)", []any{int64(2), int64(3)}},

		// Pipe filters: min, max
		{"min", "input.items | min", int64(1)},
		{"max", "input.items | max", int64(5)},

		// Pipe filters: keys, values
		{"keys", `{b: 2, a: 1} | keys`, []any{"a", "b"}},

		// Pipe filters: has
		{"has true", `{name: "Alice"} | has("name")`, true},
		{"has false", `{name: "Alice"} | has("age")`, false},

		// Pipe filters: string ops
		{"upper", `"hello" | upper`, "HELLO"},
		{"lower", `"HELLO" | lower`, "hello"},
		{"trim", `"  hi  " | trim`, "hi"},
		{"split", `"a,b,c" | split(",")`, []any{"a", "b", "c"}},
		{"replace", `"hello world" | replace("world", "Go")`, "hello Go"},
		{"truncate", `"hello world" | truncate(5)`, "hello..."},

		// Pipe filters: type conversion
		{"toInt", `"42" | toInt`, int64(42)},
		{"toFloat", `"3.14" | toFloat`, 3.14},
		{"toString", `42 | toString`, "42"},
		{"toBool", `1 | toBool`, true},

		// Pipe filters: isEmpty
		{"isEmpty nil", "null | isEmpty", true},
		{"isEmpty empty string", `"" | isEmpty`, true},
		{"isEmpty non-empty", `"hi" | isEmpty`, false},
		{"isEmpty empty array", "[] | isEmpty", true},

		// Pipe filters: default
		{"default nil", `null | default("fallback")`, "fallback"},
		{"default present", `"value" | default("fallback")`, "value"},

		// Pipe filters: json
		{"json", `[1, 2] | json`, "[1,2]"},

		// Pipe filters: round
		{"round", "3.7 | round", int64(4)},
		{"round with precision", "3.14159 | round(2)", 3.14},

		// Pipe filters: concat
		{"concat", "[1, 2] | concat([3, 4])", []any{int64(1), int64(2), int64(3), int64(4)}},

		// Method call syntax
		{"method length", "input.items.length()", int64(5)},

		// Map with lambda
		{"map lambda", "input.items | map(x => x * 2)", []any{int64(2), int64(4), int64(6), int64(8), int64(10)}},

		// Complex expression
		{"complex", `input.user?.name ?? "unknown"`, "Bob"},

		// Operator precedence
		{"precedence mul add", "2 + 3 * 4", int64(14)},
		{"precedence parens", "(2 + 3) * 4", int64(20)},

		// Chained pipes
		{"chained pipes", "input.items | filter(x => x > 2) | length", int64(3)},

		// replaceAll
		{"replaceAll", `"aabbaa" | replaceAll("aa", "x")`, "xbbx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, ctx)
			if err != nil {
				t.Fatalf("Eval(%q) error: %v", tt.expr, err)
			}
			if !deepEqual(got, tt.want) {
				t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tt.expr, got, got, tt.want, tt.want)
			}
		})
	}
}

// --- Template tests ---

func TestRenderTemplate(t *testing.T) {
	ctx := &Context{
		Input: map[string]any{
			"name":   "Alice",
			"age":    int64(30),
			"active": true,
			"items":  []any{"go", "rust", "python"},
			"empty":  []any{},
			"nested": []any{
				map[string]any{"name": "A", "tags": []any{"x", "y"}},
				map[string]any{"name": "B", "tags": []any{"z"}},
			},
		},
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "simple interpolation",
			template: "Hello, {{input.name}}!",
			want:     "Hello, Alice!",
		},
		{
			name:     "multiple interpolations",
			template: "{{input.name}} is {{input.age}} years old",
			want:     "Alice is 30 years old",
		},
		{
			name:     "pipe in interpolation",
			template: "{{input.name | upper}}",
			want:     "ALICE",
		},
		{
			name:     "if true",
			template: "{{#if input.active}}Active{{/if}}",
			want:     "Active",
		},
		{
			name:     "if false",
			template: "{{#if false}}Yes{{/if}}",
			want:     "",
		},
		{
			name:     "if else true",
			template: "{{#if input.active}}Active{{#else}}Inactive{{/if}}",
			want:     "Active",
		},
		{
			name:     "if else false",
			template: "{{#if false}}Active{{#else}}Inactive{{/if}}",
			want:     "Inactive",
		},
		{
			name:     "if with expression",
			template: `{{#if input.age > 18}}Adult{{#else}}Minor{{/if}}`,
			want:     "Adult",
		},
		{
			name:     "each basic",
			template: "{{#each input.items}}{{this}} {{/each}}",
			want:     "go rust python ",
		},
		{
			name:     "each with index",
			template: "{{#each input.items}}{{@index}}:{{this}} {{/each}}",
			want:     "0:go 1:rust 2:python ",
		},
		{
			name:     "each with first last",
			template: "{{#each input.items}}{{#if @first}}[{{/if}}{{this}}{{#if @last}}]{{/if}}{{/each}}",
			want:     "[gorustpython]",
		},
		{
			name:     "nested blocks",
			template: "{{#each input.nested}}{{this.name}}: {{#each this.tags}}{{this}}{{/each}} {{/each}}",
			want:     "A: xy B: z ",
		},
		{
			name:     "raw block",
			template: "{{raw}}{{not evaluated}}{{/raw}}",
			want:     "{{not evaluated}}",
		},
		{
			name:     "truthy empty array",
			template: "{{#if input.empty}}yes{{#else}}no{{/if}}",
			want:     "no",
		},
		{
			name:     "truthy non-empty string",
			template: `{{#if input.name}}yes{{#else}}no{{/if}}`,
			want:     "yes",
		},
		{
			name:     "truthy nil",
			template: "{{#if input.missing}}yes{{#else}}no{{/if}}",
			want:     "no",
		},
		{
			name:     "expression in template",
			template: "{{input.age + 5}}",
			want:     "35",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderTemplate(tt.template, ctx)
			if err != nil {
				t.Fatalf("RenderTemplate() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("RenderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- tokenEstimate filter tests ---

func TestTokenEstimate_String(t *testing.T) {
	// 20 chars / 4 = 5 tokens
	ctx := &Context{Input: map[string]any{"text": "12345678901234567890"}}
	result, err := Eval(`input.text | tokenEstimate`, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != int64(5) {
		t.Errorf("expected 5, got %v (%T)", result, result)
	}
}

func TestTokenEstimate_MessageArray(t *testing.T) {
	msgs := []any{
		map[string]any{"role": "user", "content": "12345678"},  // 8 chars
		map[string]any{"role": "assistant", "content": "1234"}, // 4 chars
	}
	// total 12 chars / 4 = 3 tokens
	ctx := &Context{Input: map[string]any{"messages": msgs}}
	result, err := Eval(`input.messages | tokenEstimate`, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != int64(3) {
		t.Errorf("expected 3, got %v (%T)", result, result)
	}
}

func TestTokenEstimate_Nil(t *testing.T) {
	ctx := &Context{Input: map[string]any{"x": nil}}
	result, err := Eval(`input.x | tokenEstimate`, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != int64(0) {
		t.Errorf("expected 0, got %v", result)
	}
}

// deepEqual compares two values handling []any and map[string]any.
func deepEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return reflect.DeepEqual(a, b)
}
