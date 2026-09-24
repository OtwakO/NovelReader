package epubstore

import (
	"context"

	"github.com/otwako/novelreader/internal/epub"
)

// Preview reads are authorized by the current ready receipt, not publication.
// Callers hold the reader-home lease through delivery, as with published reads.
func (s *Store) currentPreview(ctx context.Context, id string, generation int64) error {
	receipt, err := s.GetImport(ctx, id)
	if err != nil {
		return err
	}
	if receipt.State != Acquired || receipt.PreparationGeneration != generation || receipt.PreparationState != PreparationReady {
		return ErrStateChanged
	}
	return nil
}

func (s *Store) PreviewNavigation(ctx context.Context, id string, generation int64) (epub.ResolvedNavigation, error) {
	if err := s.currentPreview(ctx, id, generation); err != nil {
		return epub.ResolvedNavigation{}, err
	}
	metadata, err := s.PreparedMetadata(ctx, id, generation)
	if err != nil {
		return epub.ResolvedNavigation{}, err
	}
	if err = s.currentPreview(ctx, id, generation); err != nil {
		return epub.ResolvedNavigation{}, err
	}
	return metadata.Navigation, nil
}

func (s *Store) PreviewSection(ctx context.Context, id string, generation int64, index int) (SectionContent, error) {
	if err := s.currentPreview(ctx, id, generation); err != nil {
		return SectionContent{}, err
	}
	content, err := s.readPreparedSection(ctx, id, generation, index)
	if err != nil {
		return SectionContent{}, err
	}
	if err = s.currentPreview(ctx, id, generation); err != nil {
		return SectionContent{}, err
	}
	return content, nil
}

func (s *Store) PreviewResource(ctx context.Context, id string, generation int64, resourceID string) ([]byte, string, error) {
	if err := s.currentPreview(ctx, id, generation); err != nil {
		return nil, "", err
	}
	data, mediaType, err := s.readPreparedResource(ctx, id, generation, resourceID)
	if err != nil {
		return nil, "", err
	}
	if err = s.currentPreview(ctx, id, generation); err != nil {
		return nil, "", err
	}
	return data, mediaType, nil
}
