package api

import (
	"context"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

type readerRuntime struct {
	home               *readerstore.Home
	sourceStore        *booksource.Store
	bookStore          *book.Store
	searcher           *book.Searcher
	fontStore          *fontstore.Store
	sourceProfiles     *sourceprofile.Store
	sourceInteractions *sourceinteraction.Describer
	browserSessions    *sourceinteraction.BrowserSessions
	catalogs           *book.Catalogs
	lastUsed           time.Time
	references         int
	closing            chan struct{}
	closeErr           error
}

func (m *readerRuntimeManager) openRuntime(ctx context.Context, userID readerstore.UserID) (*readerRuntime, error) {
	home, err := m.readers.Open(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("api: open reader home: %w", err)
	}
	sourceStore := booksource.NewStore(home.DB())
	bookStore := book.NewStore(home.DB())
	fontStore := fontstore.NewStore(home.DB(), home.Files())
	sourceProfiles := sourceprofile.NewStore(home.DB(), home.CredentialsDB())
	if err := sourceProfiles.Reconcile(ctx); err != nil {
		_ = home.Close()
		return nil, fmt.Errorf("api: reconcile source profiles: %w", err)
	}
	readerJS := m.jsVM.ForkStateWithDeviceID(readerstore.DeviceID(userID))
	readerSearcher := m.searcher.ForkReader(readerJS, analyzer.NewCacheManager(), sourceStore, bookStore, m.limits)
	readerSearcher.SetSourceSessionHydrator(sourceSessionHydrator(sourceProfiles))
	return &readerRuntime{
		home: home, sourceStore: sourceStore, bookStore: bookStore, fontStore: fontStore, sourceProfiles: sourceProfiles,
		sourceInteractions: sourceinteraction.NewDescriber(sourceStore, sourceProfiles, readerJS.ForkState()),
		browserSessions:    sourceinteraction.NewBrowserSessions(m.browser),
		catalogs:           book.NewCatalogs(bookStore, sourceStore, readerSearcher), searcher: readerSearcher,
	}, nil
}

func (r *readerRuntime) close() error {
	if r.browserSessions != nil {
		r.browserSessions.CloseSource(context.Background(), "")
	}
	if r.catalogs != nil {
		r.catalogs.Close()
	}
	return r.home.Close()
}
