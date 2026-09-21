package library_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"epub-reader/internal/library"
)

func createTestEPUBAt(t *testing.T, path string, title, author string) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	f, _ := zw.Create("mimetype")
	f.Write([]byte("application/epub+zip"))

	f, _ = zw.Create("META-INF/container.xml")
	f.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	f, _ = zw.Create("content.opf")
	f.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<package version="2.0" xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
    <dc:creator>` + author + `</dc:creator>
  </metadata>
  <manifest>
    <item id="c1" href="c1.html" media-type="text/html"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
  </spine>
</package>`))

	f, _ = zw.Create("c1.html")
	f.Write([]byte(`<html><body>Content</body></html>`))

	zw.Close()

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write epub file %s: %v", path, err)
	}
}

func TestScanner(t *testing.T) {
	tmpDir := t.TempDir()

	book1 := filepath.Join(tmpDir, "book1.epub")
	createTestEPUBAt(t, book1, "Scanner Book 1", "Author 1")

	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	book2 := filepath.Join(subDir, "book2.epub")
	createTestEPUBAt(t, book2, "Scanner Book 2", "Author 2")

	// Also add a non-epub file that should be ignored
	nonEpub := filepath.Join(tmpDir, "notes.txt")
	os.WriteFile(nonEpub, []byte("some notes"), 0644)

	// Test ScanFolder
	books, err := library.ScanFolder(tmpDir)
	if err != nil {
		t.Fatalf("ScanFolder failed: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books scanned, got %d", len(books))
	}

	// Test ScanFile
	meta, err := library.ScanFile(book1)
	if err != nil {
		t.Fatalf("ScanFile failed: %v", err)
	}
	if meta.Title != "Scanner Book 1" || meta.Author != "Author 1" {
		t.Errorf("unexpected meta from ScanFile: %+v", meta)
	}
}
