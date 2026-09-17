package epub

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Optional real-book proof: no original content/references are logged or saved
// outside test-owned scratch. The emitter counts one tree at a time.
func TestLocalPreparation(t *testing.T) {
	original, size := openLocalEPUB(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	scratch := preparationScratch(t)
	var emitted, payloadBytes, images int
	result, err := Prepare(ctx, original, size, scratch, func(section PreparedSection) error {
		data, err := json.Marshal(section.Root)
		emitted++
		payloadBytes += len(data)
		images += len(section.Images)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	main := 0
	for _, section := range result.Sections {
		if section.Main {
			main++
		}
	}
	info, err := scratch.Stat()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("staged preparation: sections=%d main=%d auxiliary=%d image bindings=%d tree JSON bytes=%d scratch bytes=%d diagnostics=%v", emitted, main, emitted-main, images, payloadBytes, info.Size(), result.Diagnostics)
}
