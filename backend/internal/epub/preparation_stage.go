package epub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

const (
	maxStagedSectionBytes    int64 = 64 << 20
	maxStagedBytes           int64 = 2 << 30
	maxPreparationTitleBytes       = 4 << 20
)

type stageSpan struct{ offset, length int64 }

type preparationStage struct {
	file  io.ReadWriteSeeker
	spans []stageSpan
	bytes int64
}

func newPreparationStage(file io.ReadWriteSeeker) (*preparationStage, error) {
	end, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("epub: inspect scratch: %w", err)
	}
	if end != 0 {
		return nil, fmt.Errorf("epub: scratch must be empty")
	}
	return &preparationStage{file: file}, nil
}

func (s *preparationStage) write(ctx context.Context, section Section) error {
	writer := &stageWriter{ctx: ctx, target: s.file, remaining: min(maxStagedSectionBytes, maxStagedBytes-s.bytes)}
	if err := json.NewEncoder(writer).Encode(section); err != nil {
		return fmt.Errorf("epub: stage section: %w", err)
	}
	s.spans = append(s.spans, stageSpan{offset: s.bytes, length: writer.written})
	s.bytes += writer.written
	return ctx.Err()
}

func (s *preparationStage) read(ctx context.Context, ordinal int) (Section, error) {
	if err := ctx.Err(); err != nil {
		return Section{}, err
	}
	span := s.spans[ordinal]
	if _, err := s.file.Seek(span.offset, io.SeekStart); err != nil {
		return Section{}, fmt.Errorf("epub: seek staged section: %w", err)
	}
	var section Section
	err := json.NewDecoder(io.LimitReader(contextReader{ctx: ctx, source: s.file}, span.length)).Decode(&section)
	if err != nil {
		return Section{}, fmt.Errorf("epub: read staged section: %w", err)
	}
	return section, ctx.Err()
}

type stageWriter struct {
	ctx                context.Context
	target             io.Writer
	remaining, written int64
}

func (w *stageWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(data)) > w.remaining {
		return 0, ErrLimit
	}
	n, err := w.target.Write(data)
	w.written += int64(n)
	w.remaining -= int64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}
