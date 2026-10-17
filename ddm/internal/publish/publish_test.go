package publish

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ddm/internal/links"
	"ddm/internal/spec"
)

const specSrc = `
domains: [cathe.dev]
workflows:
  w: { steps: [ { run: x } ] }
sites:
  a: { workflow: w, mounts: [ { host: a.cathe.dev } ] }
  b: { workflow: w, mounts: [ { host: b.cathe.dev } ] }
`

func setup(t *testing.T, body string) Request {
	t.Helper()
	s, err := spec.Parse([]byte(specSrc))
	if err != nil {
		t.Fatal(err)
	}
	from := t.TempDir()
	os.WriteFile(filepath.Join(from, "index.html"), []byte(body), 0o644)
	clock := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	return Request{
		Site: "a", View: "public", Commit: "0123456789abcdef", From: from, WWW: t.TempDir(),
		Links: links.Resolver{Spec: s, Base: links.Prod}, Log: &bytes.Buffer{},
		Now: func() time.Time { clock = clock.Add(time.Second); return clock },
	}
}

func TestPublish(t *testing.T) {
	r := setup(t, `<a href="site://b/x.html">b</a>`)
	dest, err := Run(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := "20261017T120001.000-0123456789ab"; filepath.Base(dest) != want {
		t.Errorf("publish dir %s, want %s", filepath.Base(dest), want)
	}
	got, err := os.ReadFile(filepath.Join(Current(r.WWW, "public", "a"), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `<a href="https://b.cathe.dev/x.html">b</a>` {
		t.Errorf("served file: %s", got)
	}
	// The build itself is untouched: links are resolved in the copy.
	if src, _ := os.ReadFile(filepath.Join(r.From, "index.html")); !strings.Contains(string(src), "site://") {
		t.Errorf("build result was modified: %s", src)
	}
}

func TestPublishKeepsTheNewest(t *testing.T) {
	r := setup(t, "hi")
	r.Keep = 2
	var dests []string
	for range 4 {
		d, err := Run(r)
		if err != nil {
			t.Fatal(err)
		}
		dests = append(dests, filepath.Base(d))
	}
	entries, _ := os.ReadDir(Root(r.WWW, "public", "a"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{dests[2], dests[3], "current"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Errorf("after 4 publishes, keep 2: got %v, want %v", names, want)
	}
	if link, _ := os.Readlink(Current(r.WWW, "public", "a")); link != dests[3] {
		t.Errorf("current → %s, want %s", link, dests[3])
	}
}

func TestStrictUnresolved(t *testing.T) {
	r := setup(t, `<a href="site://nope/">x</a>`)
	r.Strict = true
	if _, err := Run(r); err == nil {
		t.Fatal("want an error for an unknown site in strict mode")
	}
	if _, err := os.Lstat(Current(r.WWW, "public", "a")); !os.IsNotExist(err) {
		t.Errorf("a failed publish must not create current: %v", err)
	}

	r.Strict = false
	if _, err := Run(r); err != nil {
		t.Fatalf("non-strict publish should only warn: %v", err)
	}
	if !strings.Contains(r.Log.(*bytes.Buffer).String(), "warning:") {
		t.Errorf("want a warning in the log")
	}
}
