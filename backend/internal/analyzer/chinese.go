// Package analyzer exposes Legado-compatible Chinese conversion to source JavaScript.
package analyzer

import (
	"sync"

	"github.com/longbridge/opencc"
)

var t2sConverter = sync.OnceValues(func() (*opencc.OpenCC, error) {
	return opencc.New("t2s")
})

func (*jsHelpers) T2S(text string) (string, error) {
	converter, err := t2sConverter()
	if err != nil {
		return "", err
	}
	return converter.Convert(text)
}
