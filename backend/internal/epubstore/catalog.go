package epubstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/library"
)

type publicationIdentity struct{ generation, revision int64 }

// A single joined read establishes both directions of the publication binding.
func (s *Store) publication(ctx context.Context, id string) (publicationIdentity, error) {
	var p publicationIdentity
	err := s.db.QueryRowContext(ctx, `SELECT f.preparation_generation,l.content_revision FROM library_items l JOIN epub_files f ON f.library_id=l.id AND f.id=l.id JOIN epub_preparations p ON p.file_id=f.id AND p.generation=f.preparation_generation WHERE l.id=? AND l.provider='epub' AND f.state='acquired' AND p.state='ready' AND p.format_version=?`, id, preparationFormatVersion).Scan(&p.generation, &p.revision)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return p, err
}
func (s *Store) currentPublication(ctx context.Context, id string, p publicationIdentity) error {
	current, err := s.publication(ctx, id)
	if errors.Is(err, ErrNotFound) || err == nil && current != p {
		return library.ErrStateChanged
	}
	return err
}

// GetCatalog decodes saved preparation metadata only; no archive parsing occurs.
func (s *Store) GetCatalog(ctx context.Context, id string) (epub.Preparation, int64, error) {
	p, err := s.publication(ctx, id)
	if err != nil {
		return epub.Preparation{}, 0, err
	}
	metadata, _, _, err := s.preparationMetadata(ctx, id, p.generation, PreparationReady)
	if err != nil {
		return epub.Preparation{}, 0, err
	}
	if err = s.currentPublication(ctx, id, p); err != nil {
		return epub.Preparation{}, 0, err
	}
	return metadata, p.revision, nil
}

type SectionInfo struct {
	Index int
	Title string
	Main  bool
}

// GetSection is the indexed location path for progress/bookmarks and content.
// It never decodes the whole publication's metadata or navigation.
func (s *Store) GetSection(ctx context.Context, id string, revision int64, index int) (SectionInfo, error) {
	p, err := s.publication(ctx, id)
	if err != nil {
		return SectionInfo{}, err
	}
	if p.revision != revision {
		return SectionInfo{}, library.ErrStateChanged
	}
	info, err := s.sectionInfo(ctx, id, p.generation, index)
	if err != nil {
		return SectionInfo{}, err
	}
	if err = s.currentPublication(ctx, id, p); err != nil {
		return SectionInfo{}, err
	}
	return info, nil
}
func (s *Store) sectionInfo(ctx context.Context, id string, generation int64, index int) (SectionInfo, error) {
	info := SectionInfo{Index: index}
	err := s.db.QueryRowContext(ctx, `SELECT title,main FROM epub_sections WHERE file_id=? AND generation=? AND ordinal=?`, id, generation, index).Scan(&info.Title, &info.Main)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return info, err
}

type SectionContent struct {
	Title   string
	Section epub.PreparedSection
	// Section-local binding key -> durable opaque image resource ID. Archive paths
	// stay in storage evidence and never reach the HTTP resource issuer.
	Resources map[string]string
}

func (s *Store) ReadSection(ctx context.Context, id string, revision int64, index int) (SectionContent, error) {
	p, err := s.publication(ctx, id)
	if err != nil {
		return SectionContent{}, err
	}
	if p.revision != revision {
		return SectionContent{}, library.ErrStateChanged
	}
	content, err := s.readPreparedSection(ctx, id, p.generation, index)
	if err != nil {
		return SectionContent{}, err
	}
	if err = s.currentPublication(ctx, id, p); err != nil {
		return SectionContent{}, err
	}
	return content, nil
}

// The caller owns publication or import-generation authorization.
func (s *Store) readPreparedSection(ctx context.Context, id string, generation int64, index int) (SectionContent, error) {
	info, err := s.sectionInfo(ctx, id, generation, index)
	if err != nil {
		return SectionContent{}, err
	}
	section, err := s.PreparedSection(ctx, id, generation, index)
	if err != nil {
		return SectionContent{}, err
	}
	resources := make(map[string]string, len(section.Images))
	for key, image := range section.Images {
		var resource PreparedResource
		resource.Image.Reference = image.Reference
		err = s.db.QueryRowContext(ctx, `SELECT id,derivative_id,media_type,width,height FROM epub_resources WHERE file_id=? AND generation=? AND source_path=?`, id, generation, image.Reference.Path).Scan(&resource.ID, &resource.Image.DerivativeID, &resource.Image.Info.MediaType, &resource.Image.Info.Width, &resource.Image.Info.Height)
		if err != nil {
			return SectionContent{}, err
		}
		if resource.Image != image {
			return SectionContent{}, errIncompletePreparation
		}
		resources[key] = resource.ID
	}
	return SectionContent{Title: info.Title, Section: section, Resources: resources}, nil
}
