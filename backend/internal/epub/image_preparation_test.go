package epub

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/imageproc"
	"golang.org/x/image/webp"
)

func imagePreparationFixture(t *testing.T) []fixtureEntry {
	t.Helper()
	entries := preparationFixture(t)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 4096, 2))); err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		switch entries[i].name {
		case "book.opf":
			entries[i].body = strings.Replace(entries[i].body, `id="pic"`, `id="pic" properties="cover-image"`, 1)
		case "main.xhtml":
			entries[i].body = xhtml(`<p>Main<img src="pic.png"/><img src="pic.png"/></p>`)
		case "pic.png":
			entries[i].body = encoded.String()
		}
	}
	return entries
}

func TestPrepareImageModesShareResourceIdentity(t *testing.T) {
	data := fixtureArchive(t, imagePreparationFixture(t))
	unchanged := bytes.Clone(data)
	var originalDiagnostics []string
	for _, mode := range []ImageMode{OriginalImages, OptimizedImages} {
		emitted := 0
		var sections []PreparedSection
		options := ImageOptions{Mode: mode, MaxDerivativeBytes: 1 << 20, MaxTotalDerivativeBytes: 2 << 20}
		options.Emit = func(ctx context.Context, ref Reference, info ImageInfo, encoded []byte) (string, error) {
			if mode != OptimizedImages {
				t.Fatal("original mode emitted derivative")
			}
			emitted++
			decoded, err := webp.Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatal(err)
			}
			if ref.Path != "pic.png" || info != (ImageInfo{MediaType: "image/webp", Width: 2048, Height: 1}) || decoded.Bounds().Dx() != info.Width || decoded.Bounds().Dy() != info.Height {
				t.Fatalf("derivative evidence: %+v", info)
			}
			return "caller-resource", nil
		}
		result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(section PreparedSection) error { sections = append(sections, section); return nil }, options)
		if err != nil {
			t.Fatal(err)
		}
		if result.Cover == nil || result.ImageProcessing.Mode != mode || !bytes.Equal(data, unchanged) {
			t.Fatal("cover, mode or original ownership")
		}
		for _, section := range sections {
			for _, prepared := range section.Images {
				if prepared != *result.Cover {
					t.Fatalf("cover and repeated prose resource disagree: %+v", prepared)
				}
			}
			for _, node := range nodesOfKind(section.Root, "image") {
				if node.Width != result.Cover.Info.Width || node.Height != result.Cover.Info.Height {
					t.Fatal("prose reserved source rather than derivative dimensions")
				}
			}
		}
		if mode == OriginalImages {
			originalDiagnostics = result.Diagnostics
			if emitted != 0 || result.Cover.DerivativeID != "" || result.ImageProcessing.Backend != "" || result.Cover.Info.MediaType != "image/png" {
				t.Fatal("original mode changed")
			}
		} else {
			if emitted != 1 || result.ImageProcessing.DerivativeCount != 1 || result.ImageProcessing.DerivativeBytes <= 0 || result.ImageProcessing.Backend != imageproc.EncoderBackend() || result.ImageProcessing.Profile != optimizedImageProfile || result.Cover.DerivativeID != "caller-resource" {
				t.Fatalf("shared derivative accounting: %+v", result.ImageProcessing)
			}
			if !reflect.DeepEqual(result.Diagnostics, originalDiagnostics) {
				t.Fatal("encoder backend became a content warning")
			}
		}
	}
}

func TestPrepareDerivativeFailuresAreFatal(t *testing.T) {
	data := fixtureArchive(t, imagePreparationFixture(t))
	for _, kind := range []string{"sink", "cancel", "identity", "limit", "policy"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			options := ImageOptions{Mode: OptimizedImages, MaxDerivativeBytes: 1 << 20, MaxTotalDerivativeBytes: 2 << 20}
			want := ErrImageOutput
			options.Emit = func(context.Context, Reference, ImageInfo, []byte) (string, error) {
				switch kind {
				case "sink":
					return "", ErrImageInvalid // must not become an optional image warning
				case "cancel":
					cancel()
					return "resource", nil
				case "identity":
					return "", nil
				default:
					t.Fatal("unexpected derivative emission")
					return "", nil
				}
			}
			switch kind {
			case "cancel":
				want = context.Canceled
			case "limit":
				options.MaxDerivativeBytes = 1
				want = ErrLimit
			case "policy":
				options.Emit = nil
				want = ErrImagePolicy
			}
			result, err := Prepare(ctx, bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { t.Fatal("sections emitted after derivative failure"); return nil }, options)
			if !errors.Is(err, want) || !reflect.DeepEqual(result, Preparation{}) {
				t.Fatalf("partial preparation: %+v, %v", result, err)
			}
		})
	}
}

func TestPrepareDerivativeAggregateBudget(t *testing.T) {
	entries := imagePreparationFixture(t)
	var size int64
	for _, entry := range entries {
		if entry.name == "pic.png" {
			result, err := imageproc.Optimize(context.Background(), []byte(entry.body), "image/png", imageproc.Profile{Limits: rasterLimits(), MaxEdge: 2048, Quality: 92, MaxOutputBytes: 1 << 20})
			if err != nil {
				t.Fatal(err)
			}
			size = int64(len(result.Data))
		}
	}
	for _, secondResource := range []bool{false, true} {
		fixture := append([]fixtureEntry(nil), entries...)
		if secondResource {
			for i := range fixture {
				if fixture[i].name == "book.opf" {
					fixture[i].body = strings.Replace(fixture[i].body, "</manifest>", `<item id="second" href="second.png" media-type="image/png"/></manifest>`, 1)
				}
				if fixture[i].name == "main.xhtml" {
					fixture[i].body = xhtml(`<p>Main<img src="pic.png"/><img src="second.png"/></p>`)
				}
			}
			fixture = append(fixture, fixtureEntry{"second.png", string(rasterFixture(t, "image/png"))})
		}
		data := fixtureArchive(t, fixture)
		emitted := 0
		options := ImageOptions{Mode: OptimizedImages, MaxDerivativeBytes: 1 << 20, MaxTotalDerivativeBytes: size, Emit: func(context.Context, Reference, ImageInfo, []byte) (string, error) { emitted++; return "resource", nil }}
		_, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { return nil }, options)
		if emitted != 1 || (secondResource && !errors.Is(err, ErrLimit)) || (!secondResource && err != nil) {
			t.Fatalf("budget second=%v emitted=%d: %v", secondResource, emitted, err)
		}
	}
}
