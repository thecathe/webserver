package workflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dirSpec = `
domains: [cathe.dev, jpears.me]
private_domain: the.cathe.dev
taxa: [docs, personal]
workflows:
  w: { steps: [ { run: x } ] }
sites:
  forest:
    taxa: [personal]
    description: The forest
    views: { public: { workflow: w }, private: { workflow: w } }
    mounts:
      - { host: jpears.me, entry: /@jpears/ }
      - { host: the.cathe.dev, view: private, access: private }
  hidden:
    workflow: w
    indexed: false
    mounts: [ { host: hidden.cathe.dev } ]
  secret:
    workflow: w
    taxa: [docs]
    access: private
    mounts: [ { host: secret.the.cathe.dev } ]
`

func sitePage(t *testing.T, page string) string {
	t.Helper()
	out := t.TempDir()
	p := &Plan{Site: "docs", View: "public", Vars: Vars{Out: out}, Spec: mustSpec(t, dirSpec)}
	if err := siteDirectory(context.Background(), BuiltinArgs{Plan: p, Params: map[string]string{"page": page}, Log: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "404.html")); err != nil {
		t.Errorf("no 404.html: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSiteDirectory(t *testing.T) {
	got := sitePage(t, "directory")
	if !strings.Contains(got, `<a href="site://forest/" data-split>forest</a>: The forest`) {
		t.Errorf("forest missing from the directory:\n%s", got)
	}
	for _, absent := range []string{"hidden", "secret"} {
		if strings.Contains(got, "site://"+absent+"/") {
			t.Errorf("the directory lists %s, which is unindexed or private", absent)
		}
	}
}

func TestSiteDirectoryOneTaxon(t *testing.T) {
	out := t.TempDir()
	p := &Plan{Site: "docs", View: "public", Vars: Vars{Out: out}, Spec: mustSpec(t, dirSpec)}
	params := map[string]string{"taxon": "personal", "title": "Personal"}
	if err := siteDirectory(context.Background(), BuiltinArgs{Plan: p, Params: params, Log: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "index.html"))
	got := string(b)
	if !strings.Contains(got, "<h1>Personal</h1>") || !strings.Contains(got, "site://forest/") || strings.Contains(got, "<h2>") {
		t.Errorf("one-taxon page:\n%s", got)
	}
}

func TestSiteMap(t *testing.T) {
	got := sitePage(t, "map")
	for _, want := range []string{
		// every mount, unindexed and private ones included, with its badge
		`href="mount://jpears.me/" data-split>jpears.me</a><br><span class="muted">→ /@jpears/</span>`,
		`href="mount://hidden.cathe.dev/" data-split>hidden.cathe.dev</a>`,
		`href="mount://secret.the.cathe.dev/" data-split>secret.the.cathe.dev</a>`,
		`<span class="badge private">private</span>`,
		`<span class="badge public">public</span>`,
		"3 sites",
		"not in the directory",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("site map lacks %q", want)
		}
	}
	// Grouped by domain: subdomains of the.cathe.dev follow it.
	if i, j := strings.Index(got, "<h2>the.cathe.dev</h2>"), strings.Index(got, "<h2>secret.the.cathe.dev</h2>"); i < 0 || j < i {
		t.Errorf("hosts out of order (%d, %d)", i, j)
	}
}

// Both pages carry the split view (D52), unescaped, after their links; the
// 404 page doesn't.
func TestSplitView(t *testing.T) {
	for _, page := range []string{"directory", "map"} {
		got := sitePage(t, page)
		i, j := strings.Index(got, "data-split"), strings.Index(got, "// ddm's split view")
		if j < 0 || i < 0 || j < i {
			t.Errorf("%s: the split script is missing or comes before the links (%d, %d)", page, i, j)
		}
		if !strings.Contains(got, `a[data-split]`) || !strings.Contains(got, "#ddm-split .grip") {
			t.Errorf("%s: the split script or its styles were mangled", page)
		}
	}
	out := t.TempDir()
	p := &Plan{Site: "docs", View: "public", Vars: Vars{Out: out}, Spec: mustSpec(t, dirSpec)}
	if err := siteDirectory(context.Background(), BuiltinArgs{Plan: p, Params: map[string]string{}, Log: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "404.html")); strings.Contains(string(b), "ddm-split") {
		t.Error("the 404 page has the split view")
	}
}
