// HTML selection helpers preserve fragment roots across Go's document parser.
package analyzer

import "strings"

func contextualizeHTMLSelection(content string) (string, string) {
	selector := "body > *"
	switch lower := strings.ToLower(strings.TrimSpace(content)); {
	case strings.HasPrefix(lower, "<tr"):
		return "<table><tbody>" + content + "</tbody></table>", "tbody > tr"
	case strings.HasPrefix(lower, "<td"), strings.HasPrefix(lower, "<th"):
		return "<table><tbody><tr>" + content + "</tr></tbody></table>", "tr > td, tr > th"
	case strings.HasPrefix(lower, "<thead"), strings.HasPrefix(lower, "<tbody"), strings.HasPrefix(lower, "<tfoot"), strings.HasPrefix(lower, "<caption"), strings.HasPrefix(lower, "<colgroup"):
		return "<table>" + content + "</table>", "table > *"
	case strings.HasPrefix(lower, "<option"), strings.HasPrefix(lower, "<optgroup"):
		return "<select>" + content + "</select>", "select > *"
	default:
		return content, selector
	}
}
