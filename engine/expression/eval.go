package expression

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Context provides the variable bindings for expression evaluation.
type Context struct {
	Input map[string]any
	State map[string]any
	Meta  map[string]any
	vars  map[string]any // internal: lambda bindings
}

// clone creates a shallow copy with a new vars map for lambda scoping.
func (c *Context) clone() *Context {
	nc := &Context{
		Input: c.Input,
		State: c.State,
		Meta:  c.Meta,
		vars:  make(map[string]any),
	}
	for k, v := range c.vars {
		nc.vars[k] = v
	}
	return nc
}

// Eval parses and evaluates an expression string.
func Eval(expr string, ctx *Context) (any, error) {
	node, err := Parse(expr)
	if err != nil {
		return nil, err
	}
	return EvalExpr(node, ctx)
}

// EvalExpr evaluates an AST node in the given context.
func EvalExpr(node Expr, ctx *Context) (any, error) {
	if ctx.vars == nil {
		ctx.vars = make(map[string]any)
	}
	switch n := node.(type) {
	case *LiteralExpr:
		return n.Value, nil

	case *IdentExpr:
		return evalIdent(n, ctx)

	case *DotExpr:
		return evalDot(n, ctx)

	case *OptionalChainExpr:
		return evalOptionalChain(n, ctx)

	case *IndexExpr:
		return evalIndex(n, ctx)

	case *BinaryExpr:
		return evalBinary(n, ctx)

	case *UnaryExpr:
		return evalUnary(n, ctx)

	case *TernaryExpr:
		return evalTernary(n, ctx)

	case *NullCoalesceExpr:
		return evalNullCoalesce(n, ctx)

	case *PipeExpr:
		return evalPipe(n, ctx)

	case *CallExpr:
		return evalCall(n, ctx)

	case *LambdaExpr:
		// Return the lambda node itself; it will be applied by filter functions.
		return n, nil

	case *ArrayExpr:
		return evalArray(n, ctx)

	case *ObjectExpr:
		return evalObject(n, ctx)

	default:
		return nil, fmt.Errorf("unknown expression node type %T", node)
	}
}

func evalIdent(n *IdentExpr, ctx *Context) (any, error) {
	// Check lambda vars first
	if v, ok := ctx.vars[n.Name]; ok {
		return v, nil
	}
	switch n.Name {
	case "input":
		return anyMap(ctx.Input), nil
	case "state":
		return anyMap(ctx.State), nil
	case "meta":
		return anyMap(ctx.Meta), nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	// Unknown identifier returns nil
	return nil, nil
}

func anyMap(m map[string]any) any {
	if m == nil {
		return nil
	}
	return m
}

func evalDot(n *DotExpr, ctx *Context) (any, error) {
	obj, err := EvalExpr(n.Object, ctx)
	if err != nil {
		return nil, err
	}
	return fieldAccess(obj, n.Field), nil
}

func evalOptionalChain(n *OptionalChainExpr, ctx *Context) (any, error) {
	obj, err := EvalExpr(n.Object, ctx)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	return fieldAccess(obj, n.Field), nil
}

func fieldAccess(obj any, field string) any {
	if obj == nil {
		return nil
	}
	switch v := obj.(type) {
	case map[string]any:
		return v[field]
	}
	return nil
}

func evalIndex(n *IndexExpr, ctx *Context) (any, error) {
	obj, err := EvalExpr(n.Object, ctx)
	if err != nil {
		return nil, err
	}
	idx, err := EvalExpr(n.Index, ctx)
	if err != nil {
		return nil, err
	}

	arr, ok := toSlice(obj)
	if !ok {
		return nil, nil
	}
	i, ok := toInt(idx)
	if !ok {
		return nil, nil
	}
	if i < 0 {
		i = int64(len(arr)) + i
	}
	if i < 0 || i >= int64(len(arr)) {
		return nil, nil
	}
	return arr[i], nil
}

func evalBinary(n *BinaryExpr, ctx *Context) (any, error) {
	// Short-circuit for logical operators
	if n.Op == "&&" {
		left, err := EvalExpr(n.Left, ctx)
		if err != nil {
			return nil, err
		}
		if !isTruthy(left) {
			return left, nil
		}
		return EvalExpr(n.Right, ctx)
	}
	if n.Op == "||" {
		left, err := EvalExpr(n.Left, ctx)
		if err != nil {
			return nil, err
		}
		if isTruthy(left) {
			return left, nil
		}
		return EvalExpr(n.Right, ctx)
	}

	left, err := EvalExpr(n.Left, ctx)
	if err != nil {
		return nil, err
	}
	right, err := EvalExpr(n.Right, ctx)
	if err != nil {
		return nil, err
	}

	switch n.Op {
	case "+":
		// String concatenation
		ls, lok := left.(string)
		rs, rok := right.(string)
		if lok && rok {
			return ls + rs, nil
		}
		return numericBinOp(left, right, func(a, b float64) float64 { return a + b })

	case "-":
		return numericBinOp(left, right, func(a, b float64) float64 { return a - b })
	case "*":
		return numericBinOp(left, right, func(a, b float64) float64 { return a * b })
	case "/":
		rf, rok := toFloat(right)
		if !rok || rf == 0 {
			return nil, nil
		}
		lf, lok := toFloat(left)
		if !lok {
			return nil, nil
		}
		result := lf / rf
		// Return int if both operands were ints and result is whole
		if isIntVal(left) && isIntVal(right) && result == math.Trunc(result) {
			return int64(result), nil
		}
		return result, nil
	case "%":
		li, lok := toInt(left)
		ri, rok := toInt(right)
		if !lok || !rok || ri == 0 {
			return nil, nil
		}
		return li % ri, nil

	case "==":
		return equals(left, right), nil
	case "!=":
		return !equals(left, right), nil
	case ">":
		return compareOrd(left, right, func(c int) bool { return c > 0 }), nil
	case ">=":
		return compareOrd(left, right, func(c int) bool { return c >= 0 }), nil
	case "<":
		return compareOrd(left, right, func(c int) bool { return c < 0 }), nil
	case "<=":
		return compareOrd(left, right, func(c int) bool { return c <= 0 }), nil
	}

	return nil, fmt.Errorf("unknown binary operator %q", n.Op)
}

func evalUnary(n *UnaryExpr, ctx *Context) (any, error) {
	val, err := EvalExpr(n.Operand, ctx)
	if err != nil {
		return nil, err
	}
	if n.Op == "!" {
		return !isTruthy(val), nil
	}
	return nil, fmt.Errorf("unknown unary operator %q", n.Op)
}

func evalTernary(n *TernaryExpr, ctx *Context) (any, error) {
	cond, err := EvalExpr(n.Condition, ctx)
	if err != nil {
		return nil, err
	}
	if isTruthy(cond) {
		return EvalExpr(n.Then, ctx)
	}
	return EvalExpr(n.Else, ctx)
}

func evalNullCoalesce(n *NullCoalesceExpr, ctx *Context) (any, error) {
	left, err := EvalExpr(n.Left, ctx)
	if err != nil {
		return nil, err
	}
	if left != nil {
		return left, nil
	}
	return EvalExpr(n.Right, ctx)
}

func evalCall(n *CallExpr, ctx *Context) (any, error) {
	// If Function is an ident, treat as a filter on the first arg
	if ident, ok := n.Function.(*IdentExpr); ok {
		if len(n.Args) > 0 {
			return applyFilter(nil, ident.Name, n.Args, ctx)
		}
	}
	return nil, nil
}

func evalPipe(n *PipeExpr, ctx *Context) (any, error) {
	val, err := EvalExpr(n.Value, ctx)
	if err != nil {
		return nil, err
	}
	return applyFilter(val, n.Filter, n.Args, ctx)
}

func evalArray(n *ArrayExpr, ctx *Context) (any, error) {
	result := make([]any, len(n.Elements))
	for i, el := range n.Elements {
		v, err := EvalExpr(el, ctx)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

func evalObject(n *ObjectExpr, ctx *Context) (any, error) {
	result := make(map[string]any, len(n.Keys))
	for i, key := range n.Keys {
		v, err := EvalExpr(n.Values[i], ctx)
		if err != nil {
			return nil, err
		}
		result[key] = v
	}
	return result, nil
}

// applyFilter applies a named filter to a value with optional args.
func applyFilter(val any, name string, argExprs []Expr, ctx *Context) (any, error) {
	// Evaluate args
	args := make([]any, len(argExprs))
	// Keep raw exprs for lambdas
	rawExprs := argExprs
	for i, ae := range argExprs {
		v, err := EvalExpr(ae, ctx)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	switch name {
	case "length":
		return filterLength(val)
	case "first":
		return filterFirst(val)
	case "last":
		return filterLast(val)
	case "join":
		return filterJoin(val, args)
	case "contains":
		return filterContains(val, args)
	case "map":
		return filterMap(val, args, rawExprs, ctx)
	case "filter":
		return filterFilter(val, args, rawExprs, ctx)
	case "sum":
		return filterSum(val)
	case "min":
		return filterMinMax(val, false)
	case "max":
		return filterMinMax(val, true)
	case "sort":
		return filterSort(val)
	case "reverse":
		return filterReverse(val)
	case "flatten":
		return filterFlatten(val)
	case "slice":
		return filterSlice(val, args)
	case "unique":
		return filterUnique(val)
	case "keys":
		return filterKeys(val)
	case "values":
		return filterValues(val)
	case "has":
		return filterHas(val, args)
	case "toInt":
		return filterToInt(val)
	case "toFloat":
		return filterToFloat(val)
	case "toString":
		return filterToString(val)
	case "toBool":
		return filterToBool(val)
	case "json":
		return filterJSON(val)
	case "round":
		return filterRound(val, args)
	case "upper":
		s, _ := toString(val)
		return strings.ToUpper(s), nil
	case "lower":
		s, _ := toString(val)
		return strings.ToLower(s), nil
	case "trim":
		s, _ := toString(val)
		return strings.TrimSpace(s), nil
	case "split":
		return filterSplit(val, args)
	case "replace":
		return filterReplace(val, args, false)
	case "replaceAll":
		return filterReplace(val, args, true)
	case "truncate":
		return filterTruncate(val, args)
	case "concat":
		return filterConcat(val, args)
	case "isEmpty":
		return filterIsEmpty(val), nil
	case "default":
		if val == nil {
			if len(args) > 0 {
				return args[0], nil
			}
			return nil, nil
		}
		return val, nil
	case "tokenEstimate":
		return filterTokenEstimate(val), nil
	}
	return nil, fmt.Errorf("unknown filter %q", name)
}

// --- Filter implementations ---

func filterLength(val any) (any, error) {
	switch v := val.(type) {
	case string:
		return int64(len(v)), nil
	case []any:
		return int64(len(v)), nil
	}
	if val == nil {
		return int64(0), nil
	}
	return nil, nil
}

func filterFirst(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok || len(arr) == 0 {
		return nil, nil
	}
	return arr[0], nil
}

func filterLast(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok || len(arr) == 0 {
		return nil, nil
	}
	return arr[len(arr)-1], nil
}

func filterJoin(val any, args []any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	sep := ""
	if len(args) > 0 {
		sep, _ = toString(args[0])
	}
	strs := make([]string, len(arr))
	for i, v := range arr {
		strs[i] = toStringVal(v)
	}
	return strings.Join(strs, sep), nil
}

func filterContains(val any, args []any) (any, error) {
	if len(args) == 0 {
		return false, nil
	}
	needle := args[0]
	switch v := val.(type) {
	case string:
		ns, _ := toString(needle)
		return strings.Contains(v, ns), nil
	case []any:
		for _, el := range v {
			if equals(el, needle) {
				return true, nil
			}
		}
		return false, nil
	}
	return false, nil
}

func filterMap(val any, args []any, rawExprs []Expr, ctx *Context) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	if len(args) == 0 {
		return arr, nil
	}
	result := make([]any, len(arr))

	// Check if arg is a lambda
	if lambda, ok := args[0].(*LambdaExpr); ok {
		for i, item := range arr {
			nc := ctx.clone()
			nc.vars[lambda.Param] = item
			v, err := EvalExpr(lambda.Body, nc)
			if err != nil {
				return nil, err
			}
			result[i] = v
		}
		return result, nil
	}

	// If arg is a string, treat as field name
	if fieldName, ok := args[0].(string); ok {
		for i, item := range arr {
			result[i] = fieldAccess(item, fieldName)
		}
		return result, nil
	}

	return arr, nil
}

func filterFilter(val any, args []any, rawExprs []Expr, ctx *Context) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	if len(args) == 0 {
		return arr, nil
	}

	lambda, ok := args[0].(*LambdaExpr)
	if !ok {
		return arr, nil
	}

	var result []any
	for _, item := range arr {
		nc := ctx.clone()
		nc.vars[lambda.Param] = item
		v, err := EvalExpr(lambda.Body, nc)
		if err != nil {
			return nil, err
		}
		if isTruthy(v) {
			result = append(result, item)
		}
	}
	if result == nil {
		result = []any{}
	}
	return result, nil
}

func filterSum(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	hasFloat := false
	sum := 0.0
	for _, v := range arr {
		f, ok := toFloat(v)
		if !ok {
			continue
		}
		if !isIntVal(v) {
			hasFloat = true
		}
		sum += f
	}
	if hasFloat {
		return sum, nil
	}
	return int64(sum), nil
}

func filterMinMax(val any, isMax bool) (any, error) {
	arr, ok := toSlice(val)
	if !ok || len(arr) == 0 {
		return nil, nil
	}
	hasFloat := false
	best, bestOk := toFloat(arr[0])
	if !isIntVal(arr[0]) {
		hasFloat = true
	}
	if !bestOk {
		return nil, nil
	}
	for _, v := range arr[1:] {
		f, ok := toFloat(v)
		if !ok {
			continue
		}
		if !isIntVal(v) {
			hasFloat = true
		}
		if isMax && f > best {
			best = f
		} else if !isMax && f < best {
			best = f
		}
	}
	if hasFloat {
		return best, nil
	}
	return int64(best), nil
}

func filterSort(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	result := make([]any, len(arr))
	copy(result, arr)
	sort.SliceStable(result, func(i, j int) bool {
		fi, oki := toFloat(result[i])
		fj, okj := toFloat(result[j])
		if oki && okj {
			return fi < fj
		}
		si, oki := toString(result[i])
		sj, okj := toString(result[j])
		if oki && okj {
			return si < sj
		}
		return false
	})
	return result, nil
}

func filterReverse(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	result := make([]any, len(arr))
	for i, v := range arr {
		result[len(arr)-1-i] = v
	}
	return result, nil
}

func filterFlatten(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	var result []any
	for _, v := range arr {
		if sub, ok := v.([]any); ok {
			result = append(result, sub...)
		} else {
			result = append(result, v)
		}
	}
	return result, nil
}

func filterSlice(val any, args []any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	start := int64(0)
	end := int64(len(arr))
	if len(args) > 0 {
		s, ok := toInt(args[0])
		if ok {
			start = s
		}
	}
	if len(args) > 1 {
		e, ok := toInt(args[1])
		if ok {
			end = e
		}
	}
	if start < 0 {
		start = 0
	}
	if end > int64(len(arr)) {
		end = int64(len(arr))
	}
	if start >= end {
		return []any{}, nil
	}
	return arr[start:end], nil
}

func filterUnique(val any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	seen := make(map[string]bool)
	var result []any
	for _, v := range arr {
		key := fmt.Sprintf("%v", v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result, nil
}

func filterKeys(val any) (any, error) {
	m, ok := val.(map[string]any)
	if !ok {
		return nil, nil
	}
	keys := make([]any, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].(string) < keys[j].(string)
	})
	return keys, nil
}

func filterValues(val any) (any, error) {
	m, ok := val.(map[string]any)
	if !ok {
		return nil, nil
	}
	// Sort keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	vals := make([]any, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return vals, nil
}

func filterHas(val any, args []any) (any, error) {
	m, ok := val.(map[string]any)
	if !ok || len(args) == 0 {
		return false, nil
	}
	key, _ := toString(args[0])
	_, exists := m[key]
	return exists, nil
}

func filterToInt(val any) (any, error) {
	switch v := val.(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, nil
			}
			return int64(f), nil
		}
		return i, nil
	case bool:
		if v {
			return int64(1), nil
		}
		return int64(0), nil
	}
	return nil, nil
}

func filterToFloat(val any) (any, error) {
	f, ok := toFloat(val)
	if !ok {
		return nil, nil
	}
	return f, nil
}

func filterToString(val any) (any, error) {
	return toStringVal(val), nil
}

func filterToBool(val any) (any, error) {
	return isTruthy(val), nil
}

func filterJSON(val any) (any, error) {
	b, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func filterRound(val any, args []any) (any, error) {
	f, ok := toFloat(val)
	if !ok {
		return nil, nil
	}
	n := int64(0)
	if len(args) > 0 {
		n, _ = toInt(args[0])
	}
	pow := math.Pow(10, float64(n))
	result := math.Round(f*pow) / pow
	if n == 0 {
		return int64(result), nil
	}
	return result, nil
}

func filterSplit(val any, args []any) (any, error) {
	s, ok := toString(val)
	if !ok {
		return nil, nil
	}
	sep := ""
	if len(args) > 0 {
		sep, _ = toString(args[0])
	}
	parts := strings.Split(s, sep)
	result := make([]any, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result, nil
}

func filterReplace(val any, args []any, all bool) (any, error) {
	s, ok := toString(val)
	if !ok {
		return nil, nil
	}
	if len(args) < 2 {
		return s, nil
	}
	old, _ := toString(args[0])
	new_, _ := toString(args[1])
	if all {
		return strings.ReplaceAll(s, old, new_), nil
	}
	return strings.Replace(s, old, new_, 1), nil
}

func filterTruncate(val any, args []any) (any, error) {
	s, ok := toString(val)
	if !ok {
		return nil, nil
	}
	if len(args) == 0 {
		return s, nil
	}
	n, ok := toInt(args[0])
	if !ok {
		return s, nil
	}
	runes := []rune(s)
	if int64(len(runes)) <= n {
		return s, nil
	}
	return string(runes[:n]) + "...", nil
}

func filterConcat(val any, args []any) (any, error) {
	arr, ok := toSlice(val)
	if !ok {
		return nil, nil
	}
	result := make([]any, len(arr))
	copy(result, arr)
	for _, a := range args {
		if other, ok := toSlice(a); ok {
			result = append(result, other...)
		}
	}
	return result, nil
}

func filterIsEmpty(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case bool:
		return !v
	}
	if f, ok := toFloat(val); ok {
		return f == 0
	}
	return false
}

// filterTokenEstimate estimates the number of tokens in a value.
// Uses the standard approximation of ~4 characters per token.
// Works on strings, arrays of messages (sums content fields), and numbers.
func filterTokenEstimate(val any) any {
	switch v := val.(type) {
	case string:
		return int64(len(v) / 4)
	case []any:
		total := 0
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if content, ok := m["content"].(string); ok {
					total += len(content)
				}
			} else if s, ok := item.(string); ok {
				total += len(s)
			}
		}
		return int64(total / 4)
	case nil:
		return int64(0)
	default:
		s := fmt.Sprintf("%v", v)
		return int64(len(s) / 4)
	}
}

// --- Type helpers ---

func isTruthy(val any) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int64:
		return v != 0
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return true
	}
	return true
}

func toFloat(val any) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	return 0, false
}

func toInt(val any) (int64, bool) {
	switch v := val.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		return i, err == nil
	}
	return 0, false
}

func isIntVal(val any) bool {
	switch val.(type) {
	case int, int64:
		return true
	}
	return false
}

func toString(val any) (string, bool) {
	s, ok := val.(string)
	return s, ok
}

func toStringVal(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func toSlice(val any) ([]any, bool) {
	s, ok := val.([]any)
	return s, ok
}

func equals(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Numeric comparison across types
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		_, aIsStr := a.(string)
		_, bIsStr := b.(string)
		if !aIsStr && !bIsStr {
			return af == bf
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareOrd(a, b any, pred func(int) bool) bool {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		if af < bf {
			return pred(-1)
		} else if af > bf {
			return pred(1)
		}
		return pred(0)
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		if as < bs {
			return pred(-1)
		} else if as > bs {
			return pred(1)
		}
		return pred(0)
	}
	return false
}

func numericBinOp(left, right any, op func(float64, float64) float64) (any, error) {
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if !lok || !rok {
		return nil, nil
	}
	result := op(lf, rf)
	// Return int if both were ints and result is whole
	if isIntVal(left) && isIntVal(right) && result == math.Trunc(result) {
		return int64(result), nil
	}
	return result, nil
}
