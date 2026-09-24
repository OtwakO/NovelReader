package epubstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/epub"
)

const preparationFormatVersion = 1

func (s *Store) persistPreparationIntent(ctx context.Context, a PreparationAttempt, staged *StagedPreparation) error {
	metadata, err := json.Marshal(staged.Preparation)
	if err != nil {
		return err
	}
	var size int64
	for _, span := range staged.SectionSpans {
		size += span.Length
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE epub_preparations SET state='finalizing',format_version=?,metadata_json=?,stage_name=?,stream_size=?,updated_at=?
 WHERE file_id=? AND generation=? AND state='preparing' AND EXISTS(
 SELECT 1 FROM epub_files WHERE id=? AND state='acquired' AND preparation_generation=?)`, preparationFormatVersion, metadata, staged.Directory(), size, time.Now().UnixMilli(), a.ReceiptID, a.Generation, a.ReceiptID, a.Generation)
	if err != nil {
		return err
	}
	if err = changedOne(result); err != nil {
		return err
	}
	sections, err := tx.PrepareContext(ctx, `INSERT INTO epub_sections(file_id,generation,ordinal,offset,length,title,main) VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for ordinal, span := range staged.SectionSpans {
		if _, err = sections.ExecContext(ctx, a.ReceiptID, a.Generation, ordinal, span.Offset, span.Length, staged.Preparation.Sections[ordinal].Title, staged.Preparation.Sections[ordinal].Main); err != nil {
			return errors.Join(err, sections.Close())
		}
	}
	if err = sections.Close(); err != nil {
		return err
	}
	resources, err := tx.PrepareContext(ctx, `INSERT INTO epub_resources(file_id,generation,id,source_path,derivative_id,derivative_bytes,media_type,width,height) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, r := range staged.Resources {
		image := r.Image
		if _, err = resources.ExecContext(ctx, a.ReceiptID, a.Generation, r.ID, image.Reference.Path, image.DerivativeID, r.DerivativeBytes, image.Info.MediaType, image.Info.Width, image.Info.Height); err != nil {
			return errors.Join(err, resources.Close())
		}
	}
	if err = resources.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) preparationMetadata(ctx context.Context, id string, generation int64, state PreparationState) (epub.Preparation, string, int64, error) {
	var version int
	var data []byte
	var stage string
	var size int64
	err := s.db.QueryRowContext(ctx, `SELECT p.format_version,p.metadata_json,p.stage_name,p.stream_size FROM epub_preparations p
 JOIN epub_files f ON f.id=p.file_id AND f.preparation_generation=p.generation AND f.state='acquired'
 WHERE p.file_id=? AND p.generation=? AND p.state=?`, id, generation, state).Scan(&version, &data, &stage, &size)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err != nil {
		return epub.Preparation{}, "", 0, err
	}
	if version != preparationFormatVersion {
		return epub.Preparation{}, "", 0, fmt.Errorf("epubstore: unsupported preparation format %d", version)
	}
	var metadata epub.Preparation
	err = json.Unmarshal(data, &metadata)
	return metadata, stage, size, err
}

func (s *Store) preparedResources(ctx context.Context, id string, generation int64) ([]PreparedResource, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_path,derivative_id,derivative_bytes,media_type,width,height FROM epub_resources WHERE file_id=? AND generation=? ORDER BY id`, id, generation)
	if err != nil {
		return nil, err
	}
	var result []PreparedResource
	for rows.Next() {
		var r PreparedResource
		if err = rows.Scan(&r.ID, &r.Image.Reference.Path, &r.Image.DerivativeID, &r.DerivativeBytes, &r.Image.Info.MediaType, &r.Image.Info.Width, &r.Image.Info.Height); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		result = append(result, r)
	}
	return result, errors.Join(rows.Err(), rows.Close())
}
