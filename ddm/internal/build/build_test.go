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
