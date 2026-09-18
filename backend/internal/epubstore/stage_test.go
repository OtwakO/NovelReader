package epubstore

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func stagedFixture(t *testing.T, broken bool) []byte {
	t.Helper()
	var pic, archive bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	second := `<p>Second section</p>`
	if broken {
		second = `<script>not prose</script>`
	}
	entries := [][2]string{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"book.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="two.xhtml" media-type="application/xhtml+xml"/><item id="image" href="private/pic.png" media-type="image/png" properties="cover-image"/></manifest><spine><itemref idref="one"/><itemref idref="two"/></spine></package>`},
		{"one.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>First section</p><img src="private/pic.png"/><img src="private/pic.png"/></body></html>`},
		{"two.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body>` + second + `</body></html>`},
		{"private/pic.png", pic.String()},
	}
	z := zip.NewWriter(&archive)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry[0], Method: zip.Deflate}
		if entry[0] == "mimetype" {
			header.Method = zip.Store
		}
		w, err := z.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(entry[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func TestStageOwnsOutputAndCleanup(t *testing.T) {
	for _, mode := range []epub.ImageMode{epub.OriginalImages, epub.OptimizedImages} {
		t.Run(string(mode), func(t *testing.T) {
			path := t.TempDir()
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			data := stagedFixture(t, false)
			unchanged := bytes.Clone(data)
			staged, err := Stage(context.Background(), root, bytes.NewReader(data), int64(len(data)), mode)
			if err != nil {
				t.Fatal(err)
			}
			defer staged.Close()
			if len(staged.Preparation.Sections) != 2 || !bytes.Equal(data, unchanged) {
				t.Fatal("preparation/original changed")
			}
			attempt := filepath.Join(path, staged.Directory())
			if _, err := os.Stat(filepath.Join(attempt, "scratch")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("scratch remains: %v", err)
			}
			var section epub.PreparedSection
			encoded, err := os.ReadFile(filepath.Join(attempt, sectionFile(0)))
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(encoded, &section); err != nil {
				t.Fatal(err)
			}
			if section.Ordinal != 0 || len(section.Images) == 0 {
				t.Fatal("section evidence lost")
			}
			files, err := os.ReadDir(filepath.Join(attempt, "images"))
			if err != nil {
				t.Fatal(err)
			}
			expected := 0
			if mode == epub.OptimizedImages {
				expected = 1
			}
			if len(files) != expected || staged.Preparation.ImageProcessing.DerivativeCount != expected {
				t.Fatal("duplicated or missing resource")
			}
			for _, binding := range section.Images {
				if binding != *staged.Preparation.Cover {
					t.Fatal("cover/prose resource identity differs")
				}
				if mode == epub.OptimizedImages {
					if files[0].Name() != binding.DerivativeID+".webp" {
						t.Fatal("derivative ID does not resolve")
					}
				} else if binding.DerivativeID != "" || binding.Reference.Path != "private/pic.png" {
					t.Fatal("original reference changed")
				}
			}
			if err = staged.Close(); err != nil {
				t.Fatal(err)
			}
			if err = staged.Close(); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(path)
			if err != nil || len(entries) != 0 {
				t.Fatalf("cleanup: %v %v", entries, err)
			}
		})
	}
}

func TestStageFailureDiscardsAttemptOnly(t *testing.T) {
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = root.Mkdir("another-attempt", 0700); err != nil {
		t.Fatal(err)
	}
	data := stagedFixture(t, true)
	result, err := Stage(context.Background(), root, bytes.NewReader(data), int64(len(data)), epub.OptimizedImages)
	if !errors.Is(err, epub.ErrUnsupported) || result != nil {
		t.Fatalf("failure: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err = Stage(ctx, root, bytes.NewReader(data), int64(len(data)), epub.OriginalImages); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 1 || entries[0].Name() != "another-attempt" {
		t.Fatalf("attempt isolation: %v %v", entries, err)
	}
}

func TestSectionOutputLimit(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = root.Mkdir("sections", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = writeSection(context.Background(), root, epub.PreparedSection{}, 1); !errors.Is(err, epub.ErrLimit) {
		t.Fatal(err)
	}
}
