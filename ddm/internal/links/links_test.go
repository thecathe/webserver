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
	if err == nil || !strings.Contains(err.Error(), `a.html:1: site://nope: no site named "nope"`) {
		t.Fatalf("want an unresolved-link error, got %v", err)
	}
	if n != 1 {
		t.Errorf("the resolvable link should still be rewritten; n = %d", n)
	}
}

// An unresolved link is reported with its line and the text just before it,
// once per problem (at its first place), so a failed publish says where to
// look.
func TestRewriteUnresolvedWhere(t *testing.T) {
	r := resolver(t)
	dir := t.TempDir()
	// 40 bytes before the link would cut the last-but-15th é in half; the text
	// must start at a whole rune instead.
	long := strings.Repeat("é", 30) + "x"
	os.WriteFile(filepath.Join(dir, "a.html"), []byte("<p>one</p>\n<p>two</p>\n"+long+`<a href="site://nope/x">x</a>`+"\n"+`<a href="site://nope/y">y</a>`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.css"), []byte("a{}\n\nb{background:url(mount://nosuch.dev/i.png)}"), 0o644)
	_, err := r.Rewrite(dir, "forest", "public")
	if err == nil {
		t.Fatal("want an unresolved-link error")
	}
	got := err.Error()
	for _, want := range []string{
		`a.html:3: site://nope: no site named "nope" (near "` + strings.Repeat("é", 15) + `x<a href=\"site://nope/x")`,
		`b.css:3: mount://nosuch.dev: nothing is mounted on nosuch.dev (near "a{}\n\nb{background:url(mount://nosuch.dev/i.png")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "site://nope/y") {
		t.Errorf("the same problem should be reported once, at its first place:\n%s", got)
	}
}

// Private twins (D63) are extra mounts of the same sites. They must not become
// a site's canonical address for a public build, and a private build links to
// a twin where the target has a private view.
func TestResolveWithTwins(t *testing.T) {
	s, err := spec.Parse([]byte(`
domains: [cathe.dev]
private_domain: the.cathe.dev
workflows:
  w: { steps: [ { run: x } ] }
sites:
  devlog:
    views: { public: { workflow: w }, private: { workflow: w } }
    mounts: [ { host: devlog.cathe.dev } ]
  ddm:
    workflow: w
    mounts: [ { host: ddm.cathe.dev } ]
`))
	if err != nil {
		t.Fatal(err)
	}
	r := Resolver{Spec: s, Base: Prod}
	for _, c := range []struct{ from, view, target, want string }{
		{"devlog", "public", "devlog", "https://devlog.cathe.dev/x/"},
		{"devlog", "public", "ddm", "https://ddm.cathe.dev/x/"},
		{"ddm", "public", "ddm", "https://ddm.cathe.dev/x/"},
		{"devlog", "private", "devlog", "https://devlog.the.cathe.dev/x/"},
		{"devlog", "private", "ddm", "https://ddm.cathe.dev/x/"}, // no private view: the public address
	} {
		got, err := r.Resolve(c.from, c.view, c.target, "/x/")
		if err != nil || got != c.want {
			t.Errorf("Resolve(%s, %s, %s) = %q, %v; want %q", c.from, c.view, c.target, got, err, c.want)
		}
	}
	// A twin is a private address: a public build may never link to it.
	if got, err := r.ResolveMount("ddm", "public", "ddm.the.cathe.dev", "/"); err == nil {
		t.Errorf("a public build linked to a private twin: %q", got)
	}
	if got, err := r.ResolveMount("devlog", "private", "ddm.the.cathe.dev", "/"); err != nil || got != "https://ddm.the.cathe.dev/" {
		t.Errorf("a private build should reach a twin: %q, %v", got, err)
	}
}
