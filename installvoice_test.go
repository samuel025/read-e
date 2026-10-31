package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntry struct {
	name, body, link string
	typ              byte
}

func makeTarGz(t *testing.T, entries []tarEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o755, Linkname: e.link, Size: int64(len(e.body))}
		if e.typ != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	f.Close()
	return path
}

func TestExtractTarGz(t *testing.T) {
	dest := t.TempDir()
	archive := makeTarGz(t, []tarEntry{
		{name: "piper/", typ: tar.TypeDir},
		{name: "piper/piper", body: "#!/bin/sh\n", typ: tar.TypeReg},
		{name: "piper/lib.so", body: "lib", typ: tar.TypeReg},
		{name: "piper/lib.so.1", link: "lib.so", typ: tar.TypeSymlink},
	})
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "piper", "lib.so.1")); err != nil || string(b) != "lib" {
		t.Errorf("symlink not followed to lib.so: %q %v", b, err)
	}
	if st, err := os.Stat(filepath.Join(dest, "piper", "piper")); err != nil || st.Mode()&0o100 == 0 {
		t.Errorf("binary must stay executable: %v %v", st, err)
	}
}

func TestExtractTarGzRejectsEscapes(t *testing.T) {
	cases := map[string][]tarEntry{
		"path traversal":   {{name: "../evil", body: "x", typ: tar.TypeReg}},
		"nested traversal": {{name: "piper/../../evil", body: "x", typ: tar.TypeReg}},
		"absolute symlink": {{name: "piper/l", link: "/etc/passwd", typ: tar.TypeSymlink}},
		"escaping symlink": {{name: "piper/l", link: "../../outside", typ: tar.TypeSymlink}},
	}
	for name, entries := range cases {
		dest := t.TempDir()
		if err := extractTarGz(makeTarGz(t, entries), dest); err == nil || !strings.Contains(err.Error(), "unsafe") {
			t.Errorf("%s: want an unsafe-archive error, got %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil")); err == nil {
			t.Errorf("%s: file escaped the destination", name)
		}
	}
}

func TestDownloadVerified(t *testing.T) {
	body := "voice data"
	sum := sha256.Sum256([]byte(body))
	good := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "ok")
	if err := downloadVerified(srv.URL+"/f", dest, good, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != body {
		t.Errorf("content = %q", b)
	}

	bad := filepath.Join(dir, "bad")
	if err := downloadVerified(srv.URL+"/f", bad, strings.Repeat("0", 64), nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("want checksum mismatch, got %v", err)
	}
	if _, err := os.Stat(bad); err == nil {
		t.Error("a file that failed verification must not be kept")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".download-*")); len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
	if err := downloadVerified(srv.URL+"/missing", filepath.Join(dir, "x"), good, nil); err == nil {
		t.Error("HTTP 404 must fail")
	}
}
