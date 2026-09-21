package epub_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"epub-reader/internal/epub"
)

func createTestEPUB(t *testing.T, withNav bool, withNCX bool) string {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// mimetype
	f, err := zw.Create("mimetype")
	if err != nil {
		t.Fatalf("failed to write mimetype: %v", err)
	}
	f.Write([]byte("application/epub+zip"))

	// META-INF/container.xml
	f, err = zw.Create("META-INF/container.xml")
	if err != nil {
		t.Fatalf("failed to write container.xml: %v", err)
	}
	containerContent := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`
	f.Write([]byte(containerContent))

	// OEBPS/content.opf
	f, err = zw.Create("OEBPS/content.opf")
	if err != nil {
		t.Fatalf("failed to write content.opf: %v", err)
	}

	manifestItems := `
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="img1" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="style" href="styles.css" media-type="text/css"/>
`
	if withNav {
		manifestItems += `    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>\n`
	}
	if withNCX {
		manifestItems += `    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>\n`
	}

	opfContent := `<?xml version="1.0" encoding="UTF-8"?>
<package version="3.0" unique-identifier="pub-id" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Book Title</dc:title>
    <dc:creator>Test Author</dc:creator>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>` + manifestItems + `
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`
	f.Write([]byte(opfContent))

	// Chapter 1
	f, err = zw.Create("OEBPS/ch1.xhtml")
	if err != nil {
		t.Fatalf("failed to write ch1.xhtml: %v", err)
	}
	ch1Content := `<!DOCTYPE html>
<html>
<head><title>Chapter 1</title><link rel="stylesheet" href="styles.css"/></head>
<body>
  <h1>Chapter 1</h1>
  <p>Hello world.</p>
  <img src="images/cover.jpg" alt="Cover" />
</body>
</html>`
	f.Write([]byte(ch1Content))

	// Cover image dummy
	f, err = zw.Create("OEBPS/images/cover.jpg")
	if err != nil {
		t.Fatalf("failed to write cover.jpg: %v", err)
	}
	f.Write([]byte("fake-jpeg-bytes"))

	// Stylesheet
	f, err = zw.Create("OEBPS/styles.css")
	if err != nil {
		t.Fatalf("failed to write styles.css: %v", err)
	}
	f.Write([]byte("body { font-size: 16px; }"))

	if withNav {
		f, err = zw.Create("OEBPS/nav.xhtml")
		if err != nil {
			t.Fatalf("failed to write nav.xhtml: %v", err)
		}
		navContent := `<!DOCTYPE html>
<html xmlns:epub="http://www.idpf.org/2007/ops">
<body>
  <nav type="toc">
    <ol>
      <li><a href="ch1.xhtml">Chapter 1</a></li>
    </ol>
  </nav>
</body>
</html>`
		f.Write([]byte(navContent))
	}

	if withNCX {
		f, err = zw.Create("OEBPS/toc.ncx")
		if err != nil {
			t.Fatalf("failed to write toc.ncx: %v", err)
		}
		ncxContent := `<?xml version="1.0" encoding="UTF-8"?>
<ncx version="2005-1" xmlns="http://www.daisy.org/z3986/2005/ncx/">
  <navMap>
    <navPoint id="np-1" playOrder="1">
      <navLabel><text>NCX Chapter 1</text></navLabel>
      <content src="ch1.xhtml"/>
    </navPoint>
  </navMap>
</ncx>`
		f.Write([]byte(ncxContent))
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "test_book_*.epub")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		t.Fatalf("failed to write temp epub: %v", err)
	}
	tmpFile.Close()

	return tmpFile.Name()
}

func TestEPUBParser(t *testing.T) {
	epubPath := createTestEPUB(t, true, true)
	defer os.Remove(epubPath)

	reader, err := epub.Open(epubPath)
	if err != nil {
		t.Fatalf("failed to open test epub: %v", err)
	}
	defer reader.Close()

	// 1. Check Metadata
	info := reader.Info()
	if info.Title != "Test Book Title" {
		t.Errorf("expected title 'Test Book Title', got %q", info.Title)
	}
	if info.Author != "Test Author" {
		t.Errorf("expected author 'Test Author', got %q", info.Author)
	}
	if info.SpineCount != 1 {
		t.Errorf("expected 1 spine item, got %d", info.SpineCount)
	}
	if !strings.HasPrefix(info.CoverBase64, "data:image/jpeg;base64,") {
		t.Errorf("expected cover base64 data URI, got %q", info.CoverBase64)
	}

	// 2. Check Spine
	spine := reader.SpineItems()
	if len(spine) != 1 || spine[0].Href != "ch1.xhtml" {
		t.Errorf("unexpected spine: %+v", spine)
	}

	// 3. Check TOC
	toc := reader.TOC()
	if len(toc) != 1 {
		t.Fatalf("expected 1 TOC entry, got %d", len(toc))
	}
	if toc[0].Title != "Chapter 1" {
		t.Errorf("expected TOC title 'Chapter 1', got %q", toc[0].Title)
	}
	if toc[0].SpineIndex != 0 {
		t.Errorf("expected spine index 0, got %d", toc[0].SpineIndex)
	}

	// 4. Check ChapterHTML rewriting
	html, err := reader.ChapterHTML(0)
	if err != nil {
		t.Fatalf("failed to get ChapterHTML: %v", err)
	}
	if !strings.Contains(html, "data:image/jpeg;base64,") {
		t.Errorf("expected rewritten base64 image src in html, got: %s", html)
	}
	if !strings.Contains(html, "Chapter 1") {
		t.Errorf("expected Chapter 1 text in html, got: %s", html)
	}

	// Out of range index
	_, err = reader.ChapterHTML(99)
	if err == nil {
		t.Errorf("expected error for out of range spine index")
	}
}

func TestInvalidEPUB(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.epub")
	if err := os.WriteFile(invalidFile, []byte("not a valid zip"), 0644); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}

	_, err := epub.Open(invalidFile)
	if err == nil {
		t.Fatalf("expected error opening non-zip file, got nil")
	}
}
