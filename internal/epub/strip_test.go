package epub

import "testing"

func TestStripHTMLSelfClosingTitle(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title /><link rel="stylesheet" href="style.css" type="text/css" /></head>
<body><h1>Chapter One</h1><p>The quick brown fox.</p></body>
</html>`

	got := stripHTML(src)
	want := "Chapter One The quick brown fox."
	if got != want {
		t.Fatalf("stripHTML() = %q, want %q", got, want)
	}
}
