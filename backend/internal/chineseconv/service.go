// Package chineseconv provides display-only Chinese conversion behind a replaceable runtime boundary.
package chineseconv

import (
	"context"
	"errors"
)

const (
	ModeSimplified        = "simplified"
	ModeTraditionalTaiwan = "traditional"
)

var (
	ErrUnavailable = errors.New("Chinese conversion is unavailable in this build")
	EngineVersion  = "development"
)

type Capability struct {
	Available bool              `json:"available"`
	Engine    string            `json:"engine,omitempty"`
	Version   string            `json:"version,omitempty"`
	Presets   map[string]string `json:"presets,omitempty"`
	Modes     []string          `json:"modes"`
}

type Service interface {
	Capability() Capability
	Convert(context.Context, string, []string) ([]string, error)
	Close() error
}
