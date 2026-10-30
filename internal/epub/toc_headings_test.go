package epub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readerWithFiles(t *testing.T, files map[string]string, spine []string, toc []TOCEntry) *Reader {
	t.Helper()
	p := filepath.Join(t.TempDir(), "r.epub")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
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
	r := &Reader{zip: zr, files: map[string]*zip.File{}, toc: toc}
	for _, zf := range zr.File {
		r.files[strings.ToLower(zf.Name)] = zf
	}
	for _, h := range spine {
		r.spine = append(r.spine, SpineItem{Href: h})
	}
	return r
}

func TestAddHeadingIDs(t *testing.T) {
	in := `<h2>One</h2><p>x</p><h5 class="a">Two</h5><h3 id="keep">Three</h3>`
	got := addHeadingIDs(in)
	for _, want := range []string{`<h2 id="reader-h-0">`, `<h5 id="reader-h-1" class="a">`, `<h3 id="keep">`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if addHeadingIDs(got) != got {
		t.Error("addHeadingIDs is not idempotent")
	}
}

func TestInsertIntoTOCIsSiblingNotStaircase(t *testing.T) {
	toc := []TOCEntry{{Title: "A", SpineIndex: 0}}
	for i := 1; i <= 4; i++ {
		toc, _ = insertIntoTOC(toc, TOCEntry{Title: "c", SpineIndex: i})
	}
	if len(toc) != 5 {
		t.Fatalf("expected 5 flat entries, got %d (nested: %+v)", len(toc), toc)
	}

	// An entry after a container joins the container's children, still flat.
	toc = []TOCEntry{{Title: "Part", SpineIndex: 0, Children: []TOCEntry{{Title: "c1", SpineIndex: 1}}}}
	toc, _ = insertIntoTOC(toc, TOCEntry{Title: "c2", SpineIndex: 2})
	toc, _ = insertIntoTOC(toc, TOCEntry{Title: "c3", SpineIndex: 3})
	if len(toc) != 1 || len(toc[0].Children) != 3 {
		t.Errorf("expected Part with 3 flat children, got %+v", toc)
	}
}

func TestAugmentTOCAddsEveryHeadingInAFile(t *testing.T) {
	files := map[string]string{
		"a.xhtml":  "<h1>Intro</h1>",
		"b.xhtml":  "<html><body><h2>Story</h2><h5>THE FIRST NIGHT</h5><h5>(a subtitle)</h5></body></html>",
		"c.xhtml":  "<h1>Chapter Two</h1>",
		"d.xhtml":  "<p>no headings, a continuation</p>",
		"st.xhtml": "<p></p>",
		"e.xhtml":  "<h2>Bobok</h2><h5>NOTES</h5>",
	}
	r := readerWithFiles(t, files,
		[]string{"a.xhtml", "st.xhtml", "b.xhtml", "c.xhtml", "d.xhtml", "e.xhtml"},
		[]TOCEntry{{Title: "Intro", Href: "a.xhtml", SpineIndex: 0}, {Title: "Story", Href: "st.xhtml", SpineIndex: 1}})
	r.augmentTOCWithHeadings()

	var titles []string
	var walk func([]TOCEntry, string)
	walk = func(es []TOCEntry, indent string) {
		for _, e := range es {
			titles = append(titles, indent+e.Title)
			walk(e.Children, indent+"  ")
		}
	}
	walk(r.toc, "")
	got := strings.Join(titles, "|")
	// The stub "Story" page adopts the file's other headings instead of repeating
	// the title; chapters the book's TOC missed join the section they follow at a
	// single level (no staircase); headingless files add nothing.
	want := "Intro|Story|  THE FIRST NIGHT|  (a subtitle)|  Chapter Two|  Bobok|    NOTES"
	if got != want {
		t.Errorf("TOC = %s\nwant  %s", got, want)
	}

	// Headings without an id are reachable through the generated one.
	story := r.toc[1]
	if story.Children[0].Href != "b.xhtml#reader-h-1" {
		t.Errorf("child href = %q", story.Children[0].Href)
	}
	html, err := r.ChapterHTML(2)
	if err != nil || !strings.Contains(html, `id="reader-h-1"`) {
		t.Errorf("served chapter must carry the generated id: %v %s", err, html)
	}
}

func TestLinkPreview(t *testing.T) {
	files := map[string]string{
		"ch1.xhtml":   `<p>Text<a id="c1" href="notes.xhtml#n1">1</a></p>`,
		"notes.xhtml": `<html><body><ol><li id="n1"><p>See <i>Koch</i>, Rediscovery, p. 28. <a href="ch1.xhtml#c1">↩</a></p></li></ol><p id="n2"><a>2.</a> Second note.</p><div id="n3"><a href="ch1.xhtml#c1">1</a> Third.</div></body></html>`,
	}
	r := readerWithFiles(t, files, []string{"ch1.xhtml", "notes.xhtml"}, nil)

	cases := map[string]string{
		"notes.xhtml#n1": "See Koch, Rediscovery, p. 28.",
		"notes.xhtml#n2": "Second note.",
		"notes.xhtml#n3": "Third.",
	}
	for href, want := range cases {
		got, ok := r.LinkPreview(0, href)
		if !ok || got != want {
			t.Errorf("LinkPreview(%q) = %q, %v; want %q", href, got, ok, want)
		}
	}
	for _, href := range []string{"notes.xhtml", "notes.xhtml#missing", "nope.xhtml#x"} {
		if _, ok := r.LinkPreview(0, href); ok {
			t.Errorf("LinkPreview(%q) should fail", href)
		}
	}
}
