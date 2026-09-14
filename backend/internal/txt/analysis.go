// Package txt interprets immutable TXT bytes independently of import storage and reader state.
package txt

import (
	"errors"
)

type Encoding string

const (
	UTF8    Encoding = "utf-8"
	UTF16LE Encoding = "utf-16le"
	UTF16BE Encoding = "utf-16be"
	GB18030 Encoding = "gb18030"
	Big5    Encoding = "big5"
)

type Preset string

const (
	ChineseChapters   Preset = "chinese-chapters"
	EnglishChapters   Preset = "english-chapters"
	GeneratedSections Preset = "generated-sections"
	CustomPattern     Preset = "custom"
)

// These are analyzer/read bounds, not upload or batch policy. Bound decoded bytes
// as well as original bytes: a small encoded range can expand during decoding.
const (
	maxSectionBytes       = 128 << 10
	targetSectionBytes    = maxSectionBytes / 2
	maxSectionSourceBytes = 2 * maxSectionBytes
	// MaxInputBytes bounds both acquisition and analysis of one original.
	MaxInputBytes       = 256 << 20
	maxSections         = 50000
	maxHeadingBytes     = 512
	encodingSampleBytes = 64 << 10
	parserVersion       = 1
)

var ErrEncodingRequired = errors.New("txt: encoding is uncertain; select an encoding")

type ReviewReason string

const (
	NoHeadings        ReviewReason = "no-headings"
	FewHeadings       ReviewReason = "few-headings"
	AmbiguousHeadings ReviewReason = "ambiguous-headings"
)

// Empty fields request conservative automatic encoding/rule selection.
// UTF-16 originals require a matching BOM, even with explicit selection.
// GeneratedSections is also an explicit no-heading interpretation.
type Options struct {
	Encoding Encoding
	Preset   Preset
	Pattern  string // Exact Go/RE2 expression; only used with CustomPattern.
}

var ErrInvalidOptions = errors.New("txt: unsupported interpretation options")

// Section ranges include headings and whitespace, exclude the initial BOM, and
// partition the original bytes. They are not progress/bookmark coordinates.
type Section struct {
	Title     string
	Start     int64
	End       int64
	Generated bool
}

type Analysis struct {
	Encoding      Encoding
	Preset        Preset
	ParserVersion int
	Sections      []Section
	ReviewReasons []ReviewReason
}
