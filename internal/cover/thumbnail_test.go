package cover

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func pngDataURI(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestThumbnailDownscalesLargeCover(t *testing.T) {
	in := pngDataURI(t, 1600, 2400)
	out := Thumbnail(in)

	if !strings.HasPrefix(out, "data:image/jpeg;base64,") {
		t.Fatalf("expected jpeg data URI, got prefix %q", out[:30])
	}
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(out, "data:image/jpeg;base64,"))
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != MaxWidth || cfg.Height != 720 {
		t.Errorf("expected %dx720, got %dx%d", MaxWidth, cfg.Width, cfg.Height)
	}
}

func TestThumbnailKeepsSmallAndInvalidInput(t *testing.T) {
	small := pngDataURI(t, 200, 300)
	if Thumbnail(small) != small {
		t.Error("small cover should be unchanged")
	}
	for _, in := range []string{"", "not a uri", "data:image/png;base64,!!!", "data:image/svg+xml;base64,PHN2Zy8+"} {
		if Thumbnail(in) != in {
			t.Errorf("input %q should be unchanged", in)
		}
	}
}
