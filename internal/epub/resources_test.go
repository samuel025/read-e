package epub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newResourceReader(t *testing.T) *Reader {
	t.Helper()
	p := filepath.Join(t.TempDir(), "r.epub")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{"OEBPS/img/a b.png": "PNGDATA", "OEBPS/ch2.xhtml": "<p>two</p>"} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { zr.Close() })
	r := &Reader{zip: zr, files: map[string]*zip.File{}}
	for _, zf := range zr.File {
		r.files[strings.ToLower(zf.Name)] = zf
	}
	return r
}

func TestRewriteResources(t *testing.T) {
	r := newResourceReader(t)
	in := `<img src="img/a%20b.png"><img src='img/a b.png'><a href="ch2.xhtml">x</a><a href="#n">y</a>`

	inline := r.rewriteResources(in, "OEBPS")
	if strings.Count(inline, "data:image/png;base64,") != 2 {
		t.Errorf("expected both quote styles inlined: %s", inline)
	}
	if !strings.Contains(inline, `href="ch2.xhtml"`) {
		t.Errorf("chapter link must not be inlined: %s", inline)
	}

	r.SetResourceBase("/epub/b1/")
	byURL := r.rewriteResources(in, "OEBPS")
	if strings.Contains(byURL, "base64") || strings.Count(byURL, `/epub/b1/OEBPS/img/a%20b.png`) != 2 {
		t.Errorf("expected image URLs: %s", byURL)
	}
	data, mime, err := r.ReadResource("OEBPS/img/a b.png")
	if err != nil || string(data) != "PNGDATA" || mime != "image/png" {
		t.Errorf("ReadResource = %q %q %v", data, mime, err)
	}
	if _, _, err := r.ReadResource("OEBPS/ch2.xhtml"); err == nil {
		t.Error("non-image resources must not be served")
	}
}
