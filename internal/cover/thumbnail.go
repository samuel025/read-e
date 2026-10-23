package cover

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// MaxWidth is the stored cover width: enough for a library card on a 2x display.
const MaxWidth = 480

// Thumbnail downscales a base64 data URI cover to at most MaxWidth pixels wide
// and re-encodes it as JPEG. Library cards only need a small image, and full
// resolution covers are expensive to decode and rescale on every repaint.
// Inputs that cannot be decoded, or are already small, are returned unchanged.
func Thumbnail(dataURI string) string {
	comma := strings.IndexByte(dataURI, ',')
	if !strings.HasPrefix(dataURI, "data:image/") || comma == -1 || !strings.Contains(dataURI[:comma], ";base64") {
		return dataURI
	}

	raw, err := base64.StdEncoding.DecodeString(dataURI[comma+1:])
	if err != nil {
		return dataURI
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return dataURI
	}

	b := src.Bounds()
	if b.Dx() <= MaxWidth {
		return dataURI
	}

	h := b.Dy() * MaxWidth / b.Dx()
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, MaxWidth, h))
	// Flatten transparency onto white since JPEG has no alpha channel.
	stddraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, stddraw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return dataURI
	}

	out := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	if len(out) >= len(dataURI) {
		return dataURI
	}
	return out
}
