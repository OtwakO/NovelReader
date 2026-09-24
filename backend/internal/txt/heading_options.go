package txt

import (
	"fmt"
	"regexp"
)

// MaxPatternBytes bounds the exact UTF-8 expression, not its character count.
const MaxPatternBytes = 2 << 10

var ErrInvalidPattern = fmt.Errorf("%w: invalid custom heading pattern", ErrInvalidOptions)

var headingRules = map[Preset]*regexp.Regexp{
	ChineseChapters: regexp.MustCompile(`^第[0-9０-９零〇一二三四五六七八九十百千万萬亿億两兩]+[章回节節卷部篇].*$`),
	EnglishChapters: regexp.MustCompile(`(?i)^chapter[ \t]+([0-9]+|[ivxlcdm]+)([ \t:.\-–—].*)?$`),
}

func (o Options) Validate() error {
	_, err := o.headingRule()
	return err
}

// Validation and analysis use the same compiler. Analyze keeps its result for
// the whole pass; it never recompiles expressions for individual input lines.
func (o Options) headingRule() (*regexp.Regexp, error) {
	if o.Encoding != "" && !validEncoding(o.Encoding) {
		return nil, fmt.Errorf("%w: encoding %q", ErrInvalidOptions, o.Encoding)
	}
	if o.Preset != CustomPattern {
		if o.Pattern != "" {
			return nil, fmt.Errorf("%w: select custom before supplying a pattern", ErrInvalidPattern)
		}
		if o.Preset != "" && o.Preset != GeneratedSections && headingRules[o.Preset] == nil {
			return nil, fmt.Errorf("%w: preset %q", ErrInvalidOptions, o.Preset)
		}
		return headingRules[o.Preset], nil
	}
	if len(o.Pattern) == 0 || len(o.Pattern) > MaxPatternBytes {
		return nil, fmt.Errorf("%w: expected 1–%d UTF-8 bytes", ErrInvalidPattern, MaxPatternBytes)
	}
	rule, err := regexp.Compile(o.Pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPattern, err)
	}
	if rule.MatchString("") {
		return nil, fmt.Errorf("%w: must not match empty text", ErrInvalidPattern)
	}
	// Longest lets full-line alternatives win over shorter prefix alternatives.
	// Compile the expression as supplied, not inside concatenated regex syntax.
	rule.Longest()
	return rule, nil
}

func matchesWholeHeading(rule *regexp.Regexp, line []byte) bool {
	match := rule.FindIndex(line)
	return len(match) == 2 && match[0] == 0 && match[1] == len(line)
}
