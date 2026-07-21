// Lenient map tests pin accepted Legado syntax and reject malformed separators.
package analyzer

import "testing"

func TestParseLenientStringMapRejectsMissingComma(t *testing.T) {
	if value, ok := ParseLenientStringMap(`{'A':'x' 'B':'y'}`); ok || value != nil {
		t.Fatalf("value=%v ok=%v", value, ok)
	}
}
