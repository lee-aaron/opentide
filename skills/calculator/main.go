// Calculator skill: evaluates mathematical expressions.
// Supports basic arithmetic: +, -, *, /, parentheses, and common functions.
// No network access needed. Zero egress.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Input struct {
	Arguments map[string]any `json:"arguments"`
}

type Output struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError("failed to read input: " + err.Error())
		return
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		writeError("invalid input JSON: " + err.Error())
		return
	}

	expr, _ := input.Arguments["expression"].(string)
	if expr == "" {
		writeError("missing 'expression' argument")
		return
	}

	result, err := evaluate(expr)
	if err != nil {
		writeError("evaluation error: " + err.Error())
		return
	}

	writeOutput(fmt.Sprintf("%s = %s", expr, formatNumber(result)))
}

// Simple recursive descent parser for math expressions.
// Grammar:
//   expr   = term (('+' | '-') term)*
//   term   = factor (('*' | '/') factor)*
//   factor = '-' factor | atom
//   atom   = number | '(' expr ')' | func '(' expr ')'

type parser struct {
	input string
	pos   int
}

func evaluate(expr string) (float64, error) {
	p := &parser{input: strings.TrimSpace(expr)}
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %c", p.pos, p.input[p.pos])
	}
	return result, nil
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *parser) parseFactor() (float64, error) {
	p.skipSpaces()
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}
	return p.parseAtom()
}

func (p *parser) parseAtom() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Parenthesized expression
	if p.input[p.pos] == '(' {
		p.pos++
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return val, nil
	}

	// Function call
	if unicode.IsLetter(rune(p.input[p.pos])) {
		start := p.pos
		for p.pos < len(p.input) && unicode.IsLetter(rune(p.input[p.pos])) {
			p.pos++
		}
		name := strings.ToLower(p.input[start:p.pos])
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != '(' {
			// Constants
			switch name {
			case "pi":
				return math.Pi, nil
			case "e":
				return math.E, nil
			default:
				return 0, fmt.Errorf("unknown constant: %s", name)
			}
		}
		p.pos++ // skip '('
		arg, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis for %s", name)
		}
		p.pos++
		return applyFunc(name, arg)
	}

	// Number
	return p.parseNumber()
}

func (p *parser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.pos
	for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9' || p.input[p.pos] == '.') {
		p.pos++
	}
	if p.pos == start {
		return 0, fmt.Errorf("expected number at position %d", p.pos)
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

func applyFunc(name string, arg float64) (float64, error) {
	switch name {
	case "sqrt":
		if arg < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(arg), nil
	case "abs":
		return math.Abs(arg), nil
	case "sin":
		return math.Sin(arg), nil
	case "cos":
		return math.Cos(arg), nil
	case "tan":
		return math.Tan(arg), nil
	case "log":
		if arg <= 0 {
			return 0, fmt.Errorf("log of non-positive number")
		}
		return math.Log10(arg), nil
	case "ln":
		if arg <= 0 {
			return 0, fmt.Errorf("ln of non-positive number")
		}
		return math.Log(arg), nil
	case "ceil":
		return math.Ceil(arg), nil
	case "floor":
		return math.Floor(arg), nil
	case "round":
		return math.Round(arg), nil
	default:
		return 0, fmt.Errorf("unknown function: %s", name)
	}
}

func formatNumber(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) {
		return fmt.Sprintf("%.0f", f)
	}
	s := fmt.Sprintf("%.10f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
