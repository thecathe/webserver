package build

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ddm/internal/spec"
)

// A site built end to end from a local git repo, with host-environment steps
// only, so the test needs git and sh but not nix.
func TestRun(t *testing.T) {
	origin := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = origin
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(origin, "page.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")
	sha := git("rev-parse", "HEAD")

	s, err := spec.Parse([]byte(`
domains: [cathe.dev]
workflows:
  copy:
    steps:
      - run: mkdir -p {{out}}/sub && cp page.txt {{out}}/sub/ && ln -s sub/page.txt {{out}}/link.txt
sites:
  s:
    repo: ` + origin + `
    workflow: copy
    mounts: [ { host: s.cathe.dev } ]
`))
	if err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	var log bytes.Buffer
	res, err := Run(context.Background(), Request{Spec: s, Site: "s", View: "public", System: t.TempDir(), Work: work, Log: &log})
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	if res.Commit != sha || res.Dir != filepath.Join(work, "out", "s", "public", sha) {
		t.Fatalf("result %+v, want commit %s", res, sha)
	}
	if b, err := os.ReadFile(filepath.Join(res.Dir, "sub", "page.txt")); err != nil || string(b) != "hello" {
		t.Fatalf("copied page: %q, %v", b, err)
	}
	if l, err := os.Readlink(filepath.Join(res.Dir, "link.txt")); err != nil || l != "sub/page.txt" {
		t.Fatalf("symlink not preserved: %q, %v", l, err)
	}

	// A dry run resolves and prints the plan but builds nothing.
	log.Reset()
	if _, err := Run(context.Background(), Request{Spec: s, Site: "s", View: "public", System: "/sys", Work: t.TempDir(), DryRun: true, Log: &log}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "cp page.txt") {
		t.Errorf("dry run didn't print the plan:\n%s", log.String())
	}
}

// A pooled workflow builds every member's checkout in one workspace, once.
func TestPool(t *testing.T) {
	repo := func(file string) string {
		dir := t.TempDir()
		for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "init"}} {
			if args[0] == "add" {
				os.WriteFile(filepath.Join(dir, file), []byte(file), 0o444) // read-only, like dune output
			}
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		return dir
	}
	a, b := repo("a.ml"), repo("b.ml")
	s, err := spec.Parse([]byte(`
domains: [cathe.dev]
workflows:
  docs:
    pool: true
    steps:
      - run: ls -R > {{out}}/listing.txt && cat dune-workspace >> {{out}}/listing.txt && cp a/a.ml {{out}}/
sites:
  index: { workflow: docs, mounts: [ { host: ocaml.cathe.dev } ] }
  a: { repo: ` + a + `, workflow: docs, mounts: [ { host: a.cathe.dev, entry: /a/ } ] }
  b: { repo: ` + b + `, workflow: docs }
`))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	var log bytes.Buffer
	res, err := Run(context.Background(), Request{Spec: s, Site: "b", View: "public", System: t.TempDir(), Work: work, Log: &log})
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	if res.Served != "@docs" || !strings.HasPrefix(res.Commit, "pool-") {
		t.Errorf("result: %+v", res)
	}
	listing, _ := os.ReadFile(filepath.Join(res.Dir, "listing.txt"))
	for _, want := range []string{"a.ml", "b.ml", "(lang dune"} {
		if !strings.Contains(string(listing), want) {
			t.Errorf("workspace lacks %q:\n%s", want, listing)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(res.Dir, "ddm-build.json"))
	if err != nil || !strings.Contains(string(manifest), `"site": "index"`) || !strings.Contains(string(manifest), `"branch": "main"`) {
		t.Errorf("manifest: %v\n%s", err, manifest)
	}
	// Results are writable, so publishing can rewrite links.
	if f, err := os.OpenFile(filepath.Join(res.Dir, "a.ml"), os.O_WRONLY, 0); err != nil {
		t.Errorf("result not writable: %v", err)
	} else {
		f.Close()
	}
}
