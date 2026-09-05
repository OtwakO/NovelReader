package api

import (
	"context"
	"net/http"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/candidate"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
)

// readerServices are assembled once by Server and borrowed by reader handlers.
// Their lifecycle belongs to Server, not to an individual reader runtime.
type readerServices struct {
	fetcher             *fetcher.Client
	processorCfg        processor.Config
	auth                *auth.HTTPHandler
	runtimes            *readerRuntimeManager
	webViewProbe        interface{ Probe(context.Context) error }
	chineseConversion   chineseconv.Service
	candidateOperations *candidate.Manager
	coverReferenceKey   []byte
	collectionLoader    *booksource.RemoteLoader
}

// readerAPI is bound to one runtime for its entire lifetime. Requests never
// rebind its dependencies; the outer authentication boundary owns their leases.
type readerAPI struct {
	*readerRuntime
	*readerServices
	mux             *http.ServeMux
	coverCacheScope string
}

func newReaderAPI(runtime *readerRuntime, services *readerServices) *readerAPI {
	a := &readerAPI{readerRuntime: runtime, readerServices: services, mux: http.NewServeMux(), coverCacheScope: "standalone"}
	if runtime.home != nil {
		a.coverCacheScope = readerstore.DeviceID(runtime.home.ID())
	}
	a.registerRoutes()
	return a
}

func (s *readerAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
