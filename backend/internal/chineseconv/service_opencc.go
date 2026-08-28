//go:build opencc_native

package chineseconv

/*
#cgo pkg-config: opencc
#include <stdlib.h>
#include <opencc.h>

static int opencc_is_error(opencc_t converter) {
	return converter == (opencc_t)-1;
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

type converterPair struct {
	simplified  C.opencc_t
	traditional C.opencc_t
}

const converterPoolSize = 4

type openCCService struct {
	converters chan converterPair
}

func openConverter(config string) (C.opencc_t, error) {
	name := C.CString(config)
	defer C.free(unsafe.Pointer(name))
	converter := C.opencc_open(name)
	if C.opencc_is_error(converter) != 0 {
		return converter, fmt.Errorf("open OpenCC config %q: %s", config, C.GoString(C.opencc_error()))
	}
	return converter, nil
}

func openPair() (converterPair, error) {
	simplified, err := openConverter("tw2sp_jieba.json")
	if err != nil {
		return converterPair{}, err
	}
	traditional, err := openConverter("s2twp_jieba.json")
	if err != nil {
		C.opencc_close(simplified)
		return converterPair{}, err
	}
	return converterPair{simplified: simplified, traditional: traditional}, nil
}

func closePair(pair converterPair) {
	C.opencc_close(pair.simplified)
	C.opencc_close(pair.traditional)
}

func New() (Service, error) {
	pairs := make([]converterPair, 0, converterPoolSize)
	for range converterPoolSize {
		pair, err := openPair()
		if err != nil {
			for _, opened := range pairs {
				closePair(opened)
			}
			return nil, err
		}
		pairs = append(pairs, pair)
	}
	service := &openCCService{converters: make(chan converterPair, converterPoolSize)}
	for _, pair := range pairs {
		service.converters <- pair
	}
	return service, nil
}

func (*openCCService) Capability() Capability {
	return Capability{
		Available: true,
		Engine:    "BYVoid/OpenCC",
		Version:   EngineVersion,
		Presets: map[string]string{
			ModeSimplified:        "tw2sp_jieba.json",
			ModeTraditionalTaiwan: "s2twp_jieba.json",
		},
		Modes: []string{ModeSimplified, ModeTraditionalTaiwan},
	}
}

func (s *openCCService) Convert(ctx context.Context, mode string, texts []string) ([]string, error) {
	var pair converterPair
	select {
	case pair = <-s.converters:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { s.converters <- pair }()

	converter := pair.simplified
	if mode == ModeTraditionalTaiwan {
		converter = pair.traditional
	} else if mode != ModeSimplified {
		return nil, fmt.Errorf("unsupported Chinese conversion mode %q", mode)
	}

	converted := make([]string, len(texts))
	for index, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input := C.CString(text)
		output := C.opencc_convert_utf8(converter, input, C.size_t(len([]byte(text))))
		C.free(unsafe.Pointer(input))
		if output == nil {
			return nil, fmt.Errorf("OpenCC conversion failed: %s", C.GoString(C.opencc_error()))
		}
		converted[index] = C.GoString(output)
		C.opencc_convert_utf8_free(output)
	}
	return converted, nil
}

func (s *openCCService) Close() error {
	for range cap(s.converters) {
		closePair(<-s.converters)
	}
	return nil
}
