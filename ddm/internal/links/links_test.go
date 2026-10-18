package links

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ddm/internal/spec"
)

const specSrc = `
domains: [cathe.dev, jpears.me]
private_domain: the.cathe.dev
workflows:
  w: { steps: [ { run: x } ] }
sites:
  forest:
    views: { public: { workflow: w }, private: { workflow: w } }
    mounts:
      - { host: forest.cathe.dev }
      - { host: jpears.me, entry: /@jpears/ }
      - { host: the.cathe.dev, view: private, access: private }
  cloakaml:
    workflow: w
    mounts: [ { host: cloakaml.cathe.dev } ]
  guide:
    workflow: w
    mounts: [ { host: docs.cathe.dev, path: guide } ]
  secret:
    workflow: w
    access: private
    mounts: [ { host: secret.the.cathe.dev } ]
`

func resolver(t *testing.T) Resolver {
	t.Helper()
	s, err := spec.Parse([]byte(specSrc))
	if err != nil {
		t.Fatal(err)
	}
	return Resolver{Spec: s, Base: Prod}
}

func TestResolve(t *testing.T) {
	r := resolver(t)
	cases := []struct {
		from, view, target, path string
		want, wantErr            string
	}{
		{"forest", "public", "cloakaml", "/Cloakaml/index.html", "https://cloakaml.cathe.dev/Cloakaml/index.html", ""},
		{"forest", "public", "cloakaml", "", "https://cloakaml.cathe.dev/", ""},
		{"cloakaml", "public", "forest", "/index/", "https://forest.cathe.dev/index/", ""},    // first mount is canonical
		{"cloakaml", "public", "guide", "/x.html", "https://docs.cathe.dev/guide/x.html", ""}, // mount path prefix
		{"forest", "private", "forest", "/index/", "https://the.cathe.dev/index/", ""},        // same view
		{"forest", "private", "cloakaml", "/", "https://cloakaml.cathe.dev/", ""},             // falls back to public
		{"forest", "private", "secret", "/", "https://secret.the.cathe.dev/", ""},             // private to private
		{"forest", "public", "secret", "/", "", "is private"},                                 // never leak a private host
		{"forest", "public", "nope", "/", "", `no site named "nope"`},
	}
	mounts := []struct {
		from, view, host, path string
		want, wantErr          string
	}{
		{"forest", "public", "jpears.me", "/@jpears/", "https://jpears.me/@jpears/", ""},
		{"forest", "public", "docs.cathe.dev", "", "https://docs.cathe.dev/", ""},
		{"forest", "private", "secret.the.cathe.dev", "/", "https://secret.the.cathe.dev/", ""},
		{"forest", "public", "secret.the.cathe.dev", "/", "", "is private"},
		{"forest", "public", "nope.cathe.dev", "/", "", "nothing is mounted"},
	}
	for _, c := range mounts {
		got, err := r.ResolveMount(c.from, c.view, c.host, c.path)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("mount://%s: want error %q, got %q, %v", c.host, c.wantErr, got, err)
			}
		} else if err != nil || got != c.want {
			t.Errorf("mount://%s%s: got %q, %v; want %q", c.host, c.path, got, err, c.want)
		}
	}
	for _, c := range cases {
		got, err := r.Resolve(c.from, c.view, c.target, c.path)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s/%s → %s: want error %q, got %q, %v", c.from, c.view, c.target, c.wantErr, got, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s/%s → site://%s%s: got %q, %v; want %q", c.from, c.view, c.target, c.path, got, err, c.want)
		}
	}
}

func TestRewrite(t *testing.T) {
	r := resolver(t)
	dir := t.TempDir()
	files := map[string]string{
		// forester: a \meta{redirect}{site://cloakaml/} tree becomes the href
		// of every link to it, in the XML the XSLT renders client-side.
		"index/index.xml": `<f:link href="site://cloakaml/" type="external">cloakaml</f:link><f:link href="site://cloakaml">bare</f:link>`,
		"page.html":       `<a href="site://forest/index/">forest</a> <a href='site://guide/a.html#x'>guide</a>`,
		"style.css":       `a { background: url(site://cloakaml/logo.svg); }`,
		"notes.txt":       `site://cloakaml/ stays: not a rewritable type`,
		"syntax.html":     `<p>Write <code>site://cloakaml/</code>, as in <a href="site://cloakaml/">this</a>.</p>`,
		"map.html":        `<a href="mount://jpears.me/@jpears/">jpears.me</a>`,
		// mkdocs' search index is page text: a documented example stays.
		"search/search_index.json": `{"docs":[{"text":"link: \"mount://nosuch.dev/\""}]}`,
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(body), 0o644)
	}
	n, err := r.Rewrite(dir, "cloakaml", "public")
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Errorf("rewrote %d links, want 7", n)
	}
	want := map[string]string{
		"index/index.xml":          `<f:link href="https://cloakaml.cathe.dev/" type="external">cloakaml</f:link><f:link href="https://cloakaml.cathe.dev/">bare</f:link>`,
		"page.html":                `<a href="https://forest.cathe.dev/index/">forest</a> <a href='https://docs.cathe.dev/guide/a.html#x'>guide</a>`,
		"style.css":                `a { background: url(https://cloakaml.cathe.dev/logo.svg); }`,
		"notes.txt":                files["notes.txt"],
		"syntax.html":              `<p>Write <code>site://cloakaml/</code>, as in <a href="https://cloakaml.cathe.dev/">this</a>.</p>`,
		"map.html":                 `<a href="https://jpears.me/@jpears/">jpears.me</a>`,
		"search/search_index.json": files["search/search_index.json"],
	}
	for name, w := range want {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		if string(got) != w {
			t.Errorf("%s:\n got %s\nwant %s", name, got, w)
		}
	}
}

func TestRewriteUnresolved(t *testing.T) {
	r := resolver(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.html"), []byte(`<a href="site://nope/">x</a> <a href="site://cloakaml/">y</a>`), 0o644)
	n, err := r.Rewrite(dir, "forest", "public")
	if err == nil || !strings.Contains(err.Error(), `a.html: site://nope: no site named "nope"`) {
		t.Fatalf("want an unresolved-link error, got %v", err)
	}
	if n != 1 {
		t.Errorf("the resolvable link should still be rewritten; n = %d", n)
	}
}
