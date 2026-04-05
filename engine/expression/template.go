package expression

import (
	"fmt"
	"strings"
)

// RenderTemplate processes a template string with {{expr}} interpolation and
// #if/#each block directives.
func RenderTemplate(template string, ctx *Context) (string, error) {
	if ctx.vars == nil {
		ctx.vars = make(map[string]any)
	}
	nodes, err := parseTemplate(template)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := renderNodes(nodes, ctx, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// --- Template AST ---

type tplNode interface {
	tplNode()
}

type tplText struct {
	text string
}

type tplExpr struct {
	expr string
}

type tplIf struct {
	condition string
	body      []tplNode
	elseBody  []tplNode
}

type tplEach struct {
	expr string
	body []tplNode
}

type tplRaw struct {
	text string
}

func (tplText) tplNode() {}
func (tplExpr) tplNode() {}
func (tplIf) tplNode()   {}
func (tplEach) tplNode() {}
func (tplRaw) tplNode()  {}

// --- Template parser ---

func parseTemplate(input string) ([]tplNode, error) {
	nodes, _, err := parseTemplateInner(input, 0, "")
	return nodes, err
}

func parseTemplateInner(input string, pos int, endTag string) ([]tplNode, int, error) {
	var nodes []tplNode

	for pos < len(input) {
		// Look for {{
		idx := strings.Index(input[pos:], "{{")
		if idx == -1 {
			// Rest is plain text
			nodes = append(nodes, &tplText{text: input[pos:]})
			pos = len(input)
			break
		}

		// Add text before the tag
		if idx > 0 {
			nodes = append(nodes, &tplText{text: input[pos : pos+idx]})
		}
		pos += idx

		// Find closing }}
		closeIdx := strings.Index(input[pos:], "}}")
		if closeIdx == -1 {
			return nil, pos, fmt.Errorf("unclosed {{ at position %d", pos)
		}

		tag := strings.TrimSpace(input[pos+2 : pos+closeIdx])
		nextPos := pos + closeIdx + 2

		// Check for end tags
		if endTag != "" {
			if tag == endTag {
				return nodes, nextPos, nil
			}
			if endTag == "/if" && tag == "#else" {
				// Parse else branch
				elseNodes, elsePos, err := parseTemplateInner(input, nextPos, "/if")
				if err != nil {
					return nil, 0, err
				}
				// Return special: attach else to the parent
				// We use a sentinel to communicate the else body up
				return nodes, elsePos, &elseResult{elseNodes: elseNodes}
			}
		}

		// Block tags
		if strings.HasPrefix(tag, "#if ") {
			condition := strings.TrimSpace(tag[4:])
			body, bodyPos, err := parseTemplateInner(input, nextPos, "/if")
			var elseBody []tplNode
			if err != nil {
				if er, ok := err.(*elseResult); ok {
					elseBody = er.elseNodes
				} else {
					return nil, 0, err
				}
			}
			nodes = append(nodes, &tplIf{condition: condition, body: body, elseBody: elseBody})
			pos = bodyPos
			continue
		}

		if strings.HasPrefix(tag, "#each ") {
			expr := strings.TrimSpace(tag[6:])
			body, bodyPos, err := parseTemplateInner(input, nextPos, "/each")
			if err != nil {
				return nil, 0, err
			}
			nodes = append(nodes, &tplEach{expr: expr, body: body})
			pos = bodyPos
			continue
		}

		if tag == "raw" {
			// Find {{/raw}}
			rawEnd := strings.Index(input[nextPos:], "{{/raw}}")
			if rawEnd == -1 {
				return nil, 0, fmt.Errorf("unclosed {{raw}} block")
			}
			nodes = append(nodes, &tplRaw{text: input[nextPos : nextPos+rawEnd]})
			pos = nextPos + rawEnd + 8 // len("{{/raw}}")
			continue
		}

		// Regular expression
		nodes = append(nodes, &tplExpr{expr: tag})
		pos = nextPos
	}

	if endTag != "" {
		return nil, pos, fmt.Errorf("missing {{%s}}", endTag)
	}

	return nodes, pos, nil
}

type elseResult struct {
	elseNodes []tplNode
}

func (e *elseResult) Error() string {
	return "else"
}

// --- Template renderer ---

func renderNodes(nodes []tplNode, ctx *Context, sb *strings.Builder) error {
	for _, node := range nodes {
		switch n := node.(type) {
		case *tplText:
			sb.WriteString(n.text)
		case *tplRaw:
			sb.WriteString(n.text)
		case *tplExpr:
			val, err := Eval(n.expr, ctx)
			if err != nil {
				return fmt.Errorf("error evaluating {{%s}}: %w", n.expr, err)
			}
			sb.WriteString(toStringVal(val))
		case *tplIf:
			val, err := Eval(n.condition, ctx)
			if err != nil {
				return fmt.Errorf("error evaluating #if condition: %w", err)
			}
			if isTruthy(val) {
				if err := renderNodes(n.body, ctx, sb); err != nil {
					return err
				}
			} else if n.elseBody != nil {
				if err := renderNodes(n.elseBody, ctx, sb); err != nil {
					return err
				}
			}
		case *tplEach:
			val, err := Eval(n.expr, ctx)
			if err != nil {
				return fmt.Errorf("error evaluating #each expression: %w", err)
			}
			arr, ok := toSlice(val)
			if !ok {
				continue
			}
			for i, item := range arr {
				nc := ctx.clone()
				nc.vars["this"] = item
				nc.vars["@index"] = int64(i)
				nc.vars["@first"] = i == 0
				nc.vars["@last"] = i == len(arr)-1
				if err := renderNodes(n.body, nc, sb); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
