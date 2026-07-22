// Explore category scripts run with the same bounded source state as page requests.
package book

import (
	"context"
	"fmt"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const exploreInfoMapKey = "__novelreader_explore_info_map"

func (s *Searcher) evaluateExploreScript(ctx context.Context, source booksource.BookSource, state *sourceexec.SourceSession, raw string) (string, error) {
	script, err := unwrapExploreScript(raw)
	if err != nil {
		return "", err
	}
	value, err := s.evaluateExploreJavaScript(ctx, source, state, script)
	return strings.TrimSpace(analyzer.ToString(value)), err
}

func (s *Searcher) evaluateExploreJavaScript(ctx context.Context, source booksource.BookSource, state *sourceexec.SourceSession, script string) (value interface{}, err error) {
	if s.jsVM == nil {
		return nil, fmt.Errorf("JavaScript engine unavailable")
	}
	scriptCtx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
	defer cancel()
	select {
	case s.searchSlots <- struct{}{}:
	case <-scriptCtx.Done():
		return nil, scriptCtx.Err()
	}
	s.capacity.activeSourceFetches.Add(1)
	s.capacity.totalSourceFetches.Add(1)
	defer func() {
		<-s.searchSlots
		s.capacity.activeSourceFetches.Add(-1)
		if err != nil {
			s.capacity.failedSources.Add(1)
		} else {
			s.capacity.completedSources.Add(1)
		}
	}()

	sourceHeaders, err := evaluateSourceHeaders(scriptCtx, s.jsVM, source, state)
	if err != nil {
		return nil, err
	}
	state.SetRequestHeaders(sourceHeaders)
	urlContext := &analyzer.URLContext{JSLib: source.JSLib}
	bindings := analyzer.URLBindings(urlContext, source.BookSourceURL, state)
	bindings["source"] = sourceContext(source)
	bindings["infoMap"] = exploreInfoMap(state)
	return analyzer.EvalURLScript(scriptCtx, s.jsVM, script, "", source.BookSourceURL, urlContext, bindings)
}

func unwrapExploreScript(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "@js:") {
		return raw[4:], nil
	}
	if strings.HasPrefix(lower, "<js>") {
		end := strings.LastIndex(lower, "</js>")
		if end < 4 {
			return "", fmt.Errorf("Explore script has no closing tag")
		}
		return raw[4:end], nil
	}
	return "", fmt.Errorf("Explore value is not a script")
}

func exploreInfoMap(state *sourceexec.SourceSession) map[string]interface{} {
	if values, ok := state.GetMemory(exploreInfoMapKey).(map[string]interface{}); ok {
		return values
	}
	values := map[string]interface{}{}
	values["save"] = func(...interface{}) {}
	state.PutMemory(exploreInfoMapKey, values)
	return values
}
