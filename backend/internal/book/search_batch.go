// Stateless cursor planning and execution for bounded source-search batches.
package book

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"context"

	"github.com/otwako/novelreader/internal/booksource"
)

const (
	searchCursorVersion = 1
	MaxSearchBatchSize  = 500
)

var ErrStaleSearchCursor = errors.New("search sources changed")

type SearchBatchOptions struct {
	Cursor      string
	Limit       int
	Concurrency int
}

type SearchBatchPlan struct {
	Offset               int
	Eligible             int
	RequestedBatchSize   int
	SourcesInBatch       int
	RequestedConcurrency int
	EffectiveConcurrency int
	Revision             string
	RetryCursor          string
	NextCursor           string
	HasMore              bool
	sources              []booksource.BookSource
}

type searchCursor struct {
	Version  int    `json:"v"`
	Offset   int    `json:"o"`
	Revision string `json:"r"`
}

func (s *Searcher) PrepareSearchBatch(options SearchBatchOptions) (SearchBatchPlan, error) {
	if options.Limit < 1 || options.Limit > MaxSearchBatchSize {
		return SearchBatchPlan{}, fmt.Errorf("search batch size must be between 1 and %d", MaxSearchBatchSize)
	}
	if options.Concurrency < 0 {
		return SearchBatchPlan{}, errors.New("search concurrency must not be negative")
	}

	candidates, err := s.searchCandidates()
	if err != nil {
		return SearchBatchPlan{}, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CustomOrder != candidates[j].CustomOrder {
			return candidates[i].CustomOrder < candidates[j].CustomOrder
		}
		if candidates[i].BookSourceName != candidates[j].BookSourceName {
			return candidates[i].BookSourceName < candidates[j].BookSourceName
		}
		return candidates[i].BookSourceURL < candidates[j].BookSourceURL
	})

	revision := searchSourceRevision(candidates)
	offset := 0
	if options.Cursor != "" {
		cursor, err := decodeSearchCursor(options.Cursor)
		if err != nil {
			return SearchBatchPlan{}, err
		}
		if cursor.Revision != revision {
			return SearchBatchPlan{}, ErrStaleSearchCursor
		}
		offset = cursor.Offset
	}
	if offset < 0 || offset > len(candidates) {
		return SearchBatchPlan{}, errors.New("search cursor offset is out of range")
	}

	end := min(offset+options.Limit, len(candidates))
	requestedConcurrency := options.Concurrency
	effectiveConcurrency := requestedConcurrency
	if effectiveConcurrency == 0 || effectiveConcurrency > s.concurrentPerSearch {
		effectiveConcurrency = s.concurrentPerSearch
	}
	if effectiveConcurrency < 1 {
		effectiveConcurrency = defaultMaxConcurrentSearch
	}

	plan := SearchBatchPlan{
		Offset:               offset,
		Eligible:             len(candidates),
		RequestedBatchSize:   options.Limit,
		SourcesInBatch:       end - offset,
		RequestedConcurrency: requestedConcurrency,
		EffectiveConcurrency: effectiveConcurrency,
		Revision:             revision,
		RetryCursor:          encodeSearchCursor(offset, revision),
		HasMore:              end < len(candidates),
		sources:              append([]booksource.BookSource(nil), candidates[offset:end]...),
	}
	if plan.HasMore {
		plan.NextCursor = encodeSearchCursor(end, revision)
	}
	return plan, nil
}

func (s *Searcher) SearchBatch(ctx context.Context, query string, plan SearchBatchPlan, onResult SearchCallback) error {
	s.capacity.activeSearches.Add(1)
	s.capacity.totalSearches.Add(1)
	defer s.capacity.activeSearches.Add(-1)
	return s.searchSources(ctx, query, plan.sources, plan.EffectiveConcurrency, onResult)
}

func searchSourceRevision(sources []booksource.BookSource) string {
	hash := sha256.New()
	var size [8]byte
	for _, source := range sources {
		binary.BigEndian.PutUint64(size[:], uint64(len(source.BookSourceURL)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(source.BookSourceURL))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func encodeSearchCursor(offset int, revision string) string {
	payload, _ := json.Marshal(searchCursor{Version: searchCursorVersion, Offset: offset, Revision: revision})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeSearchCursor(value string) (searchCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return searchCursor{}, errors.New("invalid search cursor")
	}
	var cursor searchCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.Version != searchCursorVersion || cursor.Offset < 0 || len(cursor.Revision) != sha256.Size*2 {
		return searchCursor{}, errors.New("invalid search cursor")
	}
	if _, err := hex.DecodeString(cursor.Revision); err != nil {
		return searchCursor{}, errors.New("invalid search cursor")
	}
	return cursor, nil
}
