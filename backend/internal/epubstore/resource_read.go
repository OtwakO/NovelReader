package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"path"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/library"
)

// ReadResource serves only stored validated image records for a current library
// publication. It neither prepares content nor decodes pixels. Keep the home lease
// through delivery; both original and derivative reads are individually bounded.
func (s *Store) ReadResource(ctx context.Context, id string, revision int64, resourceID string) (data []byte, mediaType string, err error) {
	p, err := s.publication(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if p.revision != revision {
		return nil, "", library.ErrStateChanged
	}
	data, mediaType, err = s.readPreparedResource(ctx, id, p.generation, resourceID)
	if err != nil {
		return nil, "", err
	}
	if err = s.currentPublication(ctx, id, p); err != nil {
		return nil, "", err
	}
	return data, mediaType, nil
}

// The caller authorizes before and after the bounded file read.
func (s *Store) readPreparedResource(ctx context.Context, id string, generation int64, resourceID string) (data []byte, mediaType string, err error) {
	if err = validateID(id); err != nil {
		return nil, "", err
	}
	var resource PreparedResource
	err = s.db.QueryRowContext(ctx, `SELECT source_path,derivative_id,derivative_bytes,media_type,width,height FROM epub_resources WHERE file_id=? AND generation=? AND id=?`, id, generation, resourceID).Scan(&resource.Image.Reference.Path, &resource.Image.DerivativeID, &resource.DerivativeBytes, &resource.Image.Info.MediaType, &resource.Image.Info.Width, &resource.Image.Info.Height)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return nil, "", err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	name := originalPath(id)
	if resource.Image.DerivativeID != "" {
		if err = validateID(resource.Image.DerivativeID); err != nil {
			return nil, "", err
		}
		if resource.DerivativeBytes <= 0 || resource.DerivativeBytes > maxDerivativeBytes {
			return nil, "", errIncompletePreparation
		}
		name = path.Join(preparationPath(id, generation), "images", resource.Image.DerivativeID+".webp")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, "", err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, "", errIncompletePreparation
	}
	if resource.Image.DerivativeID == "" {
		var size int64
		if err = s.db.QueryRowContext(ctx, `SELECT size FROM epub_files WHERE id=?`, id).Scan(&size); err != nil {
			return nil, "", err
		}
		if info.Size() != size {
			return nil, "", errIncompletePreparation
		}
		data, err = epub.ReadOriginalImage(ctx, file, size, resource.Image.Reference)
	} else {
		if info.Size() != resource.DerivativeBytes {
			return nil, "", errIncompletePreparation
		}
		data, err = io.ReadAll(io.LimitReader(file, resource.DerivativeBytes+1))
		if err == nil && int64(len(data)) != resource.DerivativeBytes {
			err = errIncompletePreparation
		}
	}
	if err != nil {
		return nil, "", err
	}
	if err = ctx.Err(); err != nil {
		return nil, "", err
	}
	return data, resource.Image.Info.MediaType, nil
}
