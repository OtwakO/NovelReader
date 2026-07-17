// Explore category parsing keeps imported Legado navigation data server-side.
package book

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
	"github.com/otwako/novelreader/internal/booksource"
)

const exploreKindURL = "url"

func exploreResultRules(source booksource.BookSource) (string, bool, error) {
	rules := parseRuleJSON(source.RuleExplore)
	if rules == nil || rules["bookList"] == "" {
		rules = parseRuleJSON(source.RuleSearch)
	}
	if rules == nil || rules["bookList"] == "" {
		return "", false, fmt.Errorf("source has no Explore book-list rules")
	}
	list := rules["bookList"]
	reverse := strings.HasPrefix(list, "-")
	if reverse || strings.HasPrefix(list, "+") {
		rules["bookList"] = strings.TrimSpace(list[1:])
	}
	data, err := json.Marshal(rules)
	return string(data), reverse, err
}

type exploreKind struct {
	Title    string          `json:"title"`
	URL      string          `json:"url"`
	Type     string          `json:"type"`
	Action   string          `json:"action"`
	Chars    []*string       `json:"chars"`
	Default  *string         `json:"default"`
	ViewName string          `json:"viewName"`
	Style    json.RawMessage `json:"style"`
}

func parseExploreKinds(raw string) ([]exploreKind, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var kinds []exploreKind
		if err := json.Unmarshal([]byte(raw), &kinds); err != nil {
			value, parseErr := parseLenientExploreValue(raw)
			if parseErr != nil {
				return nil, fmt.Errorf("explore categories: %w", parseErr)
			}
			data, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return nil, fmt.Errorf("explore categories: %w", marshalErr)
			}
			if err := json.Unmarshal(data, &kinds); err != nil {
				return nil, fmt.Errorf("explore categories: %w", err)
			}
		}
		for index := range kinds {
			if kinds[index].Type == "" {
				kinds[index].Type = exploreKindURL
			}
		}
		return kinds, nil
	}

	parts := strings.Split(strings.ReplaceAll(raw, "&&", "\n"), "\n")
	kinds := make([]exploreKind, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, "::")
		kind := exploreKind{Title: fields[0], Type: exploreKindURL}
		if len(fields) > 1 {
			kind.URL = fields[1]
		}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

func parseLenientExploreValue(raw string) (interface{}, error) {
	program, err := parser.ParseFile(nil, "explore", "("+escapeExploreStringControls(raw)+")", 0)
	if err != nil {
		return nil, err
	}
	if len(program.Body) != 1 {
		return nil, fmt.Errorf("expected one array expression")
	}
	statement, ok := program.Body[0].(*ast.ExpressionStatement)
	if !ok {
		return nil, fmt.Errorf("expected array expression")
	}
	return exploreLiteralValue(statement.Expression)
}

func exploreLiteralValue(expression ast.Expression) (interface{}, error) {
	switch value := expression.(type) {
	case *ast.ArrayLiteral:
		items := make([]interface{}, len(value.Value))
		for index, item := range value.Value {
			if item == nil {
				return nil, fmt.Errorf("array holes are unsupported")
			}
			parsed, err := exploreLiteralValue(item)
			if err != nil {
				return nil, err
			}
			items[index] = parsed
		}
		return items, nil
	case *ast.ObjectLiteral:
		object := make(map[string]interface{}, len(value.Value))
		for _, property := range value.Value {
			keyed, ok := property.(*ast.PropertyKeyed)
			if !ok || keyed.Computed || keyed.Kind != ast.PropertyKindValue {
				return nil, fmt.Errorf("non-data object property is unsupported")
			}
			key, err := exploreLiteralKey(keyed.Key)
			if err != nil {
				return nil, err
			}
			parsed, err := exploreLiteralValue(keyed.Value)
			if err != nil {
				return nil, err
			}
			object[key] = parsed
		}
		return object, nil
	case *ast.StringLiteral:
		return value.Value.String(), nil
	case *ast.NumberLiteral:
		return value.Value, nil
	case *ast.UnaryExpression:
		number, ok := value.Operand.(*ast.NumberLiteral)
		if !ok || value.Postfix || (value.Operator != token.PLUS && value.Operator != token.MINUS) {
			return nil, fmt.Errorf("non-numeric unary category value is unsupported")
		}
		if value.Operator == token.PLUS {
			return number.Value, nil
		}
		switch number := number.Value.(type) {
		case int64:
			return -number, nil
		case float64:
			return -number, nil
		default:
			return nil, fmt.Errorf("numeric category value %T is unsupported", number)
		}
	case *ast.BooleanLiteral:
		return value.Value, nil
	case *ast.NullLiteral:
		return nil, nil
	default:
		return nil, fmt.Errorf("executable category value %T is unsupported", expression)
	}
}

func exploreLiteralKey(expression ast.Expression) (string, error) {
	switch key := expression.(type) {
	case *ast.StringLiteral:
		return key.Value.String(), nil
	case *ast.Identifier:
		return key.Name.String(), nil
	case *ast.NumberLiteral:
		return fmt.Sprint(key.Value), nil
	default:
		return "", fmt.Errorf("category key %T is unsupported", expression)
	}
}

func escapeExploreStringControls(raw string) string {
	var output strings.Builder
	output.Grow(len(raw))
	var quote byte
	escaped := false
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if quote == 0 {
			output.WriteByte(char)
			if char == '\'' || char == '"' {
				quote = char
			}
			continue
		}
		if escaped {
			output.WriteByte(char)
			escaped = false
			continue
		}
		if char == '\\' {
			output.WriteByte(char)
			escaped = true
			continue
		}
		if char == quote {
			output.WriteByte(char)
			quote = 0
			continue
		}
		switch char {
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			output.WriteByte(char)
		}
	}
	return output.String()
}
