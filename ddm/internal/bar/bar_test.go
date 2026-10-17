package bar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ddm/internal/build"
	"ddm/internal/links"
	"ddm/internal/spec"
)

const specSrc = `
domains: [cathe.dev]
private_domain: the.cathe.dev
nav:
  - { title: forest, link: "mount://forest.cathe.dev/" }
  - { title: map, link: "mount://the.cathe.dev/sites/" }
workflows:
  w: { steps: [ { run: x } ] }
sites:
  forest:
    repo: me/forest
    views: { public: { workflow: w }, private: { workflow: w } }
    mounts:
      - { host: forest.cathe.dev }
      - { host: the.cathe.dev, view: private, access: private }
  map:
    workflow: w
    access: private
    mounts: [ { host: the.cathe.dev, path: sites } ]
`

func TestWrite(t *testing.T) {
	s, err := spec.Parse([]byte(specSrc))
	if err != nil {
		t.Fatal(err)
	}
	www := t.TempDir()
	if err := Write(www, s, links.Prod, false); err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(filepath.Join(Dir(www, false), "nav.json"))
	priv, _ := os.ReadFile(filepath.Join(Dir(www, true), "nav.json"))
	if strings.Contains(string(pub), "the.cathe.dev") {
		t.Errorf("public nav.json leaks a private address:\n%s", pub)
	}
	for _, want := range []string{`"url": "https://forest.cathe.dev/"`, `"url": "https://the.cathe.dev/sites/"`, `"view": "private"`} {
		if !strings.Contains(string(priv), want) {
			t.Errorf("private nav.json lacks %s:\n%s", want, priv)
		}
	}
	if _, err := os.Stat(filepath.Join(Dir(www, false), "bar.js")); err != nil {
		t.Error(err)
	}

	// A private-only build's status stays out of the public directory.
	if err := WriteStatus(www, s, Status{Served: "map", View: "public", Members: []build.Member{{Site: "map"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Dir(www, false), "status", "public", "map.json")); !os.IsNotExist(err) {
		t.Errorf("private status published publicly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(Dir(www, true), "status", "public", "map.json")); err != nil {
		t.Error(err)
	}
}

func TestPreviewShowsPrivateToPublicPages(t *testing.T) {
	s, _ := spec.Parse([]byte(specSrc))
	www := t.TempDir()
	if err := Write(www, s, links.Prod, true); err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(filepath.Join(Dir(www, false), "nav.json"))
	if !strings.Contains(string(pub), `"preview": true`) || !strings.Contains(string(pub), `"view": "private"`) {
		t.Errorf("a preview's public nav.json should carry the private mounts:\n%s", pub)
	}
}

func TestStatusHidesPrivateRepos(t *testing.T) {
	s, _ := spec.Parse([]byte(specSrc))
	defer func(f func(string) bool) { RepoIsPublic = f }(RepoIsPublic)
	RepoIsPublic = func(repo string) bool { return repo == "me/open" }
	www := t.TempDir()
	st := Status{Served: "forest", View: "public", Members: []build.Member{
		{Site: "forest", Repo: "me/forest", Branch: "main", Commit: "abc"},
		{Site: "other", Repo: "me/open", Branch: "main", Commit: "def"},
	}}
	if err := WriteStatus(www, s, st); err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(filepath.Join(Dir(www, false), "status", "public", "forest.json"))
	priv, _ := os.ReadFile(filepath.Join(Dir(www, true), "status", "public", "forest.json"))
	if strings.Contains(string(pub), "me/forest") || strings.Contains(string(pub), "abc") || !strings.Contains(string(pub), "me/open") {
		t.Errorf("public status:\n%s", pub)
	}
	if !strings.Contains(string(priv), "me/forest") {
		t.Errorf("private status should keep every repo:\n%s", priv)
	}
}

func TestWritePaths(t *testing.T) {
	www, build := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(build, "indimo"), 0o755)
	os.WriteFile(filepath.Join(build, "indimo", "index.html"), nil, 0o644)
	os.WriteFile(filepath.Join(build, "indimo", "index.xml"), nil, 0o644)
	if err := WritePaths(www, "public", "forest", build); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(Dir(www, true), "paths", "public", "forest.json"))
	if !strings.Contains(string(got), `"/indimo/index.html"`) || strings.Contains(string(got), "index.xml") {
		t.Errorf("paths:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(Dir(www, false), "paths")); !os.IsNotExist(err) {
		t.Errorf("page lists must stay out of the public directory")
	}
}
