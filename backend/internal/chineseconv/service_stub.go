//go:build !opencc_native

package chineseconv

import "context"

type unavailableService struct{}

func New() (Service, error) {
	return unavailableService{}, nil
}

func (unavailableService) Capability() Capability {
	return Capability{Available: false, Modes: []string{}}
}

func (unavailableService) Convert(context.Context, string, []string) ([]string, error) {
	return nil, ErrUnavailable
}

func (unavailableService) Close() error {
	return nil
}
