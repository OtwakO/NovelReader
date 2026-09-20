package api

import (
	"context"
	"net/http"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/candidate"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/reading"
	"github.com/otwako/novelreader/internal/txtstore"
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
	fileImports         *fileimport.Pool
	fileAdmission       *fileimport.Admission
	fileInbox           *fileInboxControls
}

// readerAPI is bound to one runtime for its entire lifetime. Requests never
// rebind its dependencies; the outer authentication boundary owns their leases.
type readerAPI struct {
	*readerRuntime
	*readerServices
	mux             *http.ServeMux
	coverCacheScope string
	reading         *reading.Service
	txtStore        *txtstore.Store
	epubStore       *epubstore.Store
}

func newReaderAPI(runtime *readerRuntime, services *readerServices) *readerAPI {
	a := &readerAPI{readerRuntime: runtime, readerServices: services, mux: http.NewServeMux(), coverCacheScope: "standalone"}
	if runtime.home != nil {
		a.coverCacheScope = readerstore.DeviceID(runtime.home.ID())
		a.txtStore = txtstore.NewStore(runtime.db, runtime.home.Files())
		a.epubStore = epubstore.NewStore(runtime.db, runtime.home.Files())
	}
	a.reading = &reading.Service{Library: runtime.libraryStore, TXT: a.txtStore, EPUB: a.epubStore, EPUBResourceHref: a.epubResourceHref,
		BookSource: &reading.BookSource{Store: runtime.bookStore, Sources: runtime.sourceStore,
			Catalogs: runtime.catalogs, Searcher: runtime.searcher, ProcessorConfig: services.processorCfg,
			ImageHref: chapterImageHref},
	}
	a.registerRoutes()
	if a.txtStore != nil && services.fileImports != nil {
		a.registerTXTReceiptRoutes()
		a.registerEPUBImportRoutes()
		if services.fileInbox != nil {
			a.registerTXTInboxRoutes()
			a.registerEPUBInboxRoutes()
		}
	}
	return a
}

func (s *readerAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
