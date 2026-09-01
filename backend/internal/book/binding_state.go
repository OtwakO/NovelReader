package book

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const bindingStateVersion = 1

// bindingState is the persisted source-binding graph for one shelf book.
// Active and alternate are roles of the same complete binding shape.
type bindingState struct {
	Version    int         `json:"version"`
	Active     AltSource   `json:"active"`
	Alternates []AltSource `json:"alternates,omitempty"`
}

func bindingFromBook(book *Book) AltSource {
	binding := AltSource{
		SourceID:    book.SourceID,
		SourceURL:   book.SourceURL,
		BookURL:     book.BookURL,
		SourceName:  book.Origin,
		LastChapter: book.LastChapter,
	}
	if book.ActiveSource != nil && sameBinding(binding, *book.ActiveSource) {
		binding = enrichAlternateSource(binding, *book.ActiveSource)
	}
	return binding
}

func bindingStateFromBook(book *Book) bindingState {
	state := bindingState{Version: bindingStateVersion, Active: bindingFromBook(book)}
	for _, alternate := range book.AlternateSources {
		state = state.upsert(alternate)
	}
	return state
}

func decodeBindingState(raw string, book *Book) (bindingState, error) {
	state := bindingState{Version: bindingStateVersion, Active: bindingFromBook(book)}
	data := bytes.TrimSpace([]byte(raw))
	if len(data) == 0 || bytes.Equal(data, []byte("[]")) {
		return state, nil
	}

	switch data[0] {
	case '[':
		var legacy []AltSource
		if err := json.Unmarshal(data, &legacy); err != nil {
			return bindingState{}, fmt.Errorf("book: decode legacy binding state: %w", err)
		}
		for _, binding := range legacy {
			state = state.upsert(binding)
		}
		return state, nil
	case '{':
		var stored bindingState
		if err := json.Unmarshal(data, &stored); err != nil {
			return bindingState{}, fmt.Errorf("book: decode binding state: %w", err)
		}
		if stored.Version != bindingStateVersion {
			return bindingState{}, fmt.Errorf("book: unsupported binding state version %d", stored.Version)
		}
		if !validBinding(stored.Active) {
			return bindingState{}, errors.New("book: binding state has invalid active binding")
		}
		if !sameBinding(state.Active, stored.Active) {
			return bindingState{}, errors.New("book: binding state active binding does not match book")
		}
		state.Active = enrichAlternateSource(state.Active, stored.Active)
		for _, alternate := range stored.Alternates {
			state = state.upsert(alternate)
		}
		return state, nil
	default:
		return bindingState{}, errors.New("book: binding state must be a JSON object or legacy array")
	}
}

func encodeBindingState(state bindingState) (string, error) {
	state.Version = bindingStateVersion
	if !validBinding(state.Active) {
		if len(state.Alternates) == 0 {
			return "[]", nil
		}
		return "", errors.New("book: cannot encode bindings without a valid active binding")
	}
	state.Alternates = mergeAlternateSources(state.Active.SourceID, state.Active.BookURL, state.Alternates)
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("book: encode binding state: %w", err)
	}
	return string(encoded), nil
}

func applyBindingState(book *Book, state bindingState) {
	book.ActiveSource = nil
	if validBinding(state.Active) {
		active := state.Active
		book.ActiveSource = &active
	}
	book.AlternateSources = append([]AltSource(nil), state.Alternates...)
}

func (state bindingState) upsert(incoming AltSource) bindingState {
	if !validBinding(incoming) {
		return state
	}
	if sameBinding(state.Active, incoming) {
		state.Active = enrichAlternateSource(state.Active, incoming)
		return state
	}
	for index := range state.Alternates {
		if sameBinding(state.Alternates[index], incoming) {
			state.Alternates[index] = enrichAlternateSource(state.Alternates[index], incoming)
			return state
		}
	}
	state.Alternates = append(state.Alternates, incoming)
	return state
}

func (state bindingState) promote(sourceID, bookURL string) (bindingState, error) {
	for index := range state.Alternates {
		if state.Alternates[index].SourceID != sourceID || state.Alternates[index].BookURL != bookURL {
			continue
		}
		selected := state.Alternates[index]
		state.Alternates[index] = state.Active
		state.Active = selected
		return state, nil
	}
	return bindingState{}, ErrSourceNotAlternate
}

func (state bindingState) clearAlternates() bindingState {
	state.Alternates = nil
	return state
}

func sameBinding(left, right AltSource) bool {
	return left.SourceID == right.SourceID && left.BookURL == right.BookURL
}

func validBinding(binding AltSource) bool {
	return binding.SourceID != "" && binding.BookURL != ""
}
