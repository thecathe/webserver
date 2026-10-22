package spec

import (
	"strings"
	"testing"
)

func TestGeneratedParsedAndDefaulted(t *testing.T) {
	src := strings.Replace(base, "    taxa: [docs]\n    mounts", `    taxa: [docs]
    generated:
      model: Opus 5.5
      date: 2026-10-16
      regions:
        - { paths: ["notes/**"], selector: "main article" }
      exclude: ["notes/drafts/**"]
    mounts`, 1)
	s, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	g := s.Sites["docs"].Generated
	if g == nil {
		t.Fatal("generated block was dropped")
	}
	if g.By != DefaultGeneratedBy || g.Model != "Opus 5.5" || g.Date != "2026-10-16" {
		t.Errorf("got %+v", g)
	}
	if s.Sites["forest"].Generated != nil {
		t.Error("a site without a block must have none")
	}
}

func TestGeneratedInvalid(t *testing.T) {
	cases := []struct{ name, block, want string }{
		{"bad date", "date: yesterday", `date "yesterday" must be a date`},
		{"bad edited", "edited: 2026-13-40", `edited "2026-13-40" must be a date`},
		{"region without paths", "regions: [ { selector: main } ]", "regions[0]: paths is required"},
		{"rooted glob", `regions: [ { paths: ["/notes/**"] } ]`, "no leading /"},
		{"dotdot glob", `exclude: ["../x"]`, "no .."},
		{"unknown field", "colour: yellow", "field colour not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := strings.Replace(base, "    taxa: [docs]\n    mounts", "    taxa: [docs]\n    generated:\n      "+c.block+"\n    mounts", 1)
			_, err := Parse([]byte(src))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestGeneratedMatcher(t *testing.T) {
	g := &Generated{
		Regions: []Region{
			{Paths: []string{"notes/**"}, Selector: "main article"},
			{Paths: []string{"about", "*.html"}},
		},
		Exclude: []string{"notes/drafts/**", "**/print.html"},
	}
	m, err := g.Compile()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		rel  string
		sel  string
		want bool
	}{
		{"notes/a/index.html", "main article", true},
		{"notes/index.html", "main article", true},
		{"notes/drafts/x/index.html", "", false},
		{"notes/a/print.html", "", false},
		{"about/index.html", "", true}, // by the directory
		{"index.html", "", true},       // *.html stays in the root
		{"other/index.html", "", false},
		{"other/page.html", "", false},
	}
	for _, c := range cases {
		sel, ok := m.For(c.rel)
		if ok != c.want || sel != c.sel {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.rel, sel, ok, c.sel, c.want)
		}
	}

	all, _ := (&Generated{}).Compile()
	if sel, ok := all.For("any/where.html"); !ok || sel != "" {
		t.Errorf("no regions should cover every page, got (%q, %v)", sel, ok)
	}
}
