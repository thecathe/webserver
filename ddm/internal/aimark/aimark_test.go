package aimark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ddm/internal/bar"
	"ddm/internal/spec"
)

func TestWrite(t *testing.T) {
	www := t.TempDir()
	if err := Write(www); err != nil {
		t.Fatal(err)
	}
	for _, private := range []bool{false, true} {
		for _, f := range []string{"ai.js", "ai.css"} {
			b, err := os.ReadFile(filepath.Join(bar.Dir(www, private), f))
			if err != nil || len(b) == 0 {
				t.Errorf("private=%v %s: %v (%d bytes)", private, f, err, len(b))
			}
		}
	}
	for _, want := range []string{"/_ddm/ai.css", "/_ddm/ai.js"} {
		if !strings.Contains(Tags, want) {
			t.Errorf("Tags should load %s: %s", want, Tags)
		}
	}
}

func TestPageHead(t *testing.T) {
	g := &spec.Generated{
		By: "Claude", Model: "Opus 5.5", Effort: "medium", Date: "2026-10-16",
		Regions: []spec.Region{{Paths: []string{"notes/**"}, Selector: `main [data-x="{1}"]`}},
		Exclude: []string{"notes/private/**"},
	}
	head, err := PageHead(g)
	if err != nil {
		t.Fatal(err)
	}
	want := `<meta name="ddm:generated-site" content="Claude" data-model="Opus 5.5" data-effort="medium" data-date="2026-10-16" data-selector="main [data-x=&#34;{1}&#34;]"/>`
	if got := head("notes/a/index.html"); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	// In an XSLT stylesheet a brace starts an expression, so it is doubled.
	if got := head("notes/a/page.xsl"); !strings.Contains(got, `{{1}}`) {
		t.Errorf("xsl should double braces: %s", got)
	}
	for _, rel := range []string{"index.html", "notes/private/x/index.html"} {
		if got := head(rel); got != "" {
			t.Errorf("%s should not be marked, got %s", rel, got)
		}
	}

	none, err := PageHead(nil)
	if err != nil || none("anything.html") != "" {
		t.Errorf("a site without a block marks nothing: %v", err)
	}
}

func TestPageHeadEscapes(t *testing.T) {
	head, _ := PageHead(&spec.Generated{By: `A "B" <c>`})
	if got := head("x.html"); strings.Contains(got, `"B"`) || strings.Contains(got, "<c>") {
		t.Errorf("author must be escaped: %s", got)
	}
}
