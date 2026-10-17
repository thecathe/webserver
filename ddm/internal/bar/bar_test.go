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
	if err := Write(www, s, links.Prod); err != nil {
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
