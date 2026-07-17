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

func (s *Searcher) evaluateExploreScript(ctx context.Context, source booksource.BookSource, state *sourceexec.SourceSession, raw string) (output string, err error) {
	if s.jsVM == nil {
		return "", fmt.Errorf("JavaScript engine unavailable")
	}
	script, err := unwrapExploreScript(raw)
	if err != nil {
		return "", err
	}
	scriptCtx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
	defer cancel()
	select {
	case s.searchSlots <- struct{}{}:
	case <-scriptCtx.Done():
		return "", scriptCtx.Err()
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

	state.SetRequestHeaders(parseHeaderJSON(source.Header))
	urlContext := &analyzer.URLContext{JSLib: source.JSLib}
	bindings := analyzer.URLBindings(urlContext, source.BookSourceURL, state)
	bindings["infoMap"] = exploreInfoMap(state)
	value, err := analyzer.EvalURLScript(scriptCtx, s.jsVM, script, "", source.BookSourceURL, urlContext, bindings)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(analyzer.ToString(value)), nil
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
