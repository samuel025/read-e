package epub

import "testing"

func TestResolveLink(t *testing.T) {
	r := &Reader{
		opfDir: "OEBPS",
		spine: []SpineItem{
			{Href: "Text/ch1.xhtml"},
			{Href: "Text/ch%202.xhtml"},
			{Href: "Notes/notes.xhtml"},
		},
	}
	cases := []struct {
		from  int
		href  string
		want  int
		frag  string
		found bool
	}{
		{0, "#fn1", 0, "fn1", true},
		{0, "ch1.xhtml#fn1", 0, "fn1", true},
		{0, "ch%202.xhtml", 1, "", true},
		{0, "ch 2.xhtml#a", 1, "a", true},
		{0, "../Notes/notes.xhtml#n3", 2, "n3", true},
		{2, "../Text/CH1.xhtml", 0, "", true},
		{0, "missing.xhtml", 0, "", false},
		{9, "#x", 0, "", false},
	}
	for _, c := range cases {
		got, frag, ok := r.ResolveLink(c.from, c.href)
		if ok != c.found || (ok && (got != c.want || frag != c.frag)) {
			t.Errorf("ResolveLink(%d, %q) = %d %q %v; want %d %q %v", c.from, c.href, got, frag, ok, c.want, c.frag, c.found)
		}
	}
}
