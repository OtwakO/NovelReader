// Lenient map tests pin accepted Legado syntax and reject malformed separators.
package analyzer

import "testing"

func TestParseLenientStringMapRejectsMalformedEntries(t *testing.T) {
	for _, raw := range []string{
		`{'A':'x' 'B':'y'}`,
		`{'A':'x',,'B':'y'}`,
		`{'A':,'B':'y'}`,
	} {
		if value, ok := ParseLenientStringMap(raw); ok || value != nil {
			t.Errorf("ParseLenientStringMap(%q)=%v, %v", raw, value, ok)
		}
	}
}
