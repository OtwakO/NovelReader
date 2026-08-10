package book

import (
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
)

func TestSearcherForkReaderSharesCapacityAndIsolatesWorkflowState(t *testing.T) {
	root := NewSearcherWithLimits(nil, analyzer.NewJSVMWithPoolSize(1), analyzer.NewCacheManager(), nil, nil, SearcherLimits{
		ConcurrentPerSearch:  1,
		ConcurrentGlobal:     1,
		MaxSessions:          2,
		SessionTTL:           time.Minute,
		WorkflowTimeout:      time.Second,
		ExploreSourceTimeout: time.Second,
	})
	first := root.ForkReader(root.jsVM.ForkState(), analyzer.NewCacheManager(), nil, nil, SearcherLimits{MaxSessions: 2, SessionTTL: time.Minute})
	second := root.ForkReader(root.jsVM.ForkState(), analyzer.NewCacheManager(), nil, nil, SearcherLimits{MaxSessions: 2, SessionTTL: time.Minute})

	if first.searchSlots != second.searchSlots || first.capacity != second.capacity {
		t.Fatal("reader searchers do not share process capacity")
	}
	firstSession := first.sessions.GetOrCreateBook("source", "book")
	firstSession.PutVariable("secret", "alice")
	secondSession := second.sessions.GetOrCreateBook("source", "book")
	if firstSession == secondSession || secondSession.GetVariable("secret") != "" {
		t.Fatal("reader searchers share workflow session state")
	}
	if first.explore == second.explore || first.jsVM == second.jsVM || first.cache == second.cache {
		t.Fatal("reader searchers share mutable reader state")
	}
}
