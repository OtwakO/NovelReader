// Lenient map parsing supports Legado's JavaScript-style source configuration objects.
package analyzer

// ParseLenientStringMap accepts quoted JSON-like string maps, including single quotes.
func ParseLenientStringMap(raw string) (map[string]string, bool) {
	return parseLenientRuleMap(raw)
}
