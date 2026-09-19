package epubstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func TestPreparedJSONLimits(t *testing.T) {
	// Strings containing delimiters/escapes are data, not nesting.
	if err := checkPreparedJSON(t.Context(), []byte(`{"text":"[{}]\\\"中文","number":1e999}`)); err != nil {
		t.Fatal(err)
	}
	atLimit := "[" + strings.Repeat("null,", maxPreparedJSONTokens-3) + "null]"
	if err := checkPreparedJSON(t.Context(), []byte(atLimit)); err != nil {
		t.Fatal("token boundary", err)
	}
	if err := checkPreparedJSON(t.Context(), []byte("[null,"+atLimit[1:])); !errors.Is(err, epub.ErrLimit) {
		t.Fatal("scalar expansion", err)
	}
	deep := strings.Repeat("[", maxPreparedJSONDepth) + "0" + strings.Repeat("]", maxPreparedJSONDepth)
	if err := checkPreparedJSON(t.Context(), []byte(deep)); err != nil {
		t.Fatal("depth boundary", err)
	}
	if err := checkPreparedJSON(t.Context(), []byte("["+deep+"]")); !errors.Is(err, epub.ErrLimit) {
		t.Fatal("depth", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := checkPreparedJSON(ctx, []byte(`{}`)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSectionJSONLimitsAtWriteAndRestore(t *testing.T) {
	section := epub.PreparedSection{}
	for i := 0; i < maxPreparedJSONDepth; i++ {
		section.Root = epub.Node{Kind: "group", Children: []epub.Node{section.Root}}
	}
	var output bytes.Buffer
	span, err := appendSection(t.Context(), &output, section, 0)
	if !errors.Is(err, epub.ErrLimit) || span != (SectionSpan{}) || output.Len() != 0 {
		t.Fatal("staging admitted deep output", span, err)
	}
	data, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{data, []byte(`{"Root":{"Children":[` + strings.Repeat("null,", maxPreparedJSONTokens) + `null]}}`)} {
		_, err = readPortableSection(t.Context(), bytes.NewReader(data), int64(len(data)), 0, SectionSpan{Length: int64(len(data))})
		if !errors.Is(err, epub.ErrLimit) {
			t.Fatal("portable typed decode was not bounded", err)
		}
	}
}

func TestPortableMetadataJSONLimits(t *testing.T) {
	store, root := receiptStore(t)
	receipt := acquiredFixture(t, store, epub.OriginalImages)
	attempt, err := store.QueuePreparation(t.Context(), receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Prepare(t.Context(), receipt.ID, attempt.Generation); err != nil {
		t.Fatal(err)
	}
	// Oversized SQL values must be rejected before transfer to a Go byte slice.
	if _, err = store.db.Exec(`UPDATE epub_preparations SET metadata_json=zeroblob(?)`, maxPreparedJSONBytes+1); err != nil {
		t.Fatal(err)
	}
	if err = checkPortableResourceFixture(t, store, root); !errors.Is(err, epub.ErrLimit) {
		t.Fatal("metadata byte budget", err)
	}
	deep := []byte(strings.Repeat("[", maxPreparedJSONDepth+1) + "0" + strings.Repeat("]", maxPreparedJSONDepth+1))
	if _, err = store.db.Exec(`UPDATE epub_preparations SET metadata_json=?`, deep); err != nil {
		t.Fatal(err)
	}
	if err = checkPortableResourceFixture(t, store, root); !errors.Is(err, epub.ErrLimit) {
		t.Fatal("metadata preflight", err)
	}
}
