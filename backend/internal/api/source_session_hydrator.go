package api

import (
	"context"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func sourceSessionHydrator(profiles *sourceprofile.Store) book.SourceSessionHydrator {
	return func(ctx context.Context, source booksource.BookSource, session *sourceexec.SourceSession) error {
		profile, err := profiles.Load(ctx, source.ID)
		if err != nil {
			return err
		}
		sourceprofile.ApplySettings(session, source.BookSourceURL, sourceprofile.DecodeSettings(profile.Settings))
		sourceprofile.ApplyAuthentication(session, sourceprofile.DecodeAuthentication(profile.Authentication))
		return nil
	}
}

func sourceAuthenticationHydrator(profiles *sourceprofile.Store) book.SourceSessionHydrator {
	return func(ctx context.Context, source booksource.BookSource, session *sourceexec.SourceSession) error {
		profile, err := profiles.Load(ctx, source.ID)
		if err != nil {
			return err
		}
		sourceprofile.ApplyAuthentication(session, sourceprofile.DecodeAuthentication(profile.Authentication))
		return nil
	}
}
