package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathFromArg(t *testing.T) {
	dir := t.TempDir()
	epub := filepath.Join(dir, "My Book.epub")
	txt := filepath.Join(dir, "notes.txt")
	for _, f := range []string{epub, txt} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string]string{
		epub:                                epub,
		"file://" + dir + "/My%20Book.epub": epub, // as passed by %U
		txt:                                 "",
		filepath.Join(dir, "missing.epub"):  "",
		dir:                                 "",
		"--some-flag":                       "",
	}
	for arg, want := range cases {
		if got := pathFromArg(arg); got != want {
			t.Errorf("pathFromArg(%q) = %q, want %q", arg, got, want)
		}
	}
}
