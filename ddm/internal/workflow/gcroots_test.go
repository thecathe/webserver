package workflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The toolchain a step runs in is kept alive by a GC root (a nix profile in the
// site's cache), named for what the environment is and not for where it is,
// because the checkout and this repo move with every commit.
func TestRootKey(t *testing.T) {
	v := Vars{Repo: "/w/src/forest", System: "/store/abc-source"}
	for _, c := range []struct{ env, want string }{
		{"/store/abc-source#mkdocs", "system-mkdocs"},
		{"/store/xyz-source#mkdocs", "other-store-xyz-source-mkdocs"}, // another sha is not this repo's: kept as written
		{"/w/src/forest", "repo"},
		{"/w/src/forest#shell", "repo-shell"},
		{"/w/src/forest-old", "other-w-src-forest-old"}, // a longer name is not under the checkout
		{"github:owner/repo#x", "other-github-owner-repo-x"},
	} {
		if got := v.rootKey(c.env); got != c.want {
			t.Errorf("rootKey(%q) = %q, want %q", c.env, got, c.want)
		}
	}
	long := strings.Repeat("a", 200)
	if got := v.rootKey(long); len(got) > 80 || got == v.rootKey(long+"b") {
		t.Errorf("a long env should be shortened and still distinct: %q", got)
	}
}

// fakeNix puts a `nix` on PATH that records its arguments and, for `develop`,
// runs the command after --command, as nix does.
func fakeNix(t *testing.T) (logFile string) {
	t.Helper()
	dir := t.TempDir()
	logFile = filepath.Join(dir, "nix.log")
	script := `#!/bin/sh
echo "$*" >> ` + logFile + `
if [ "$1" = develop ]; then
  while [ $# -gt 0 ]; do
    if [ "$1" = --command ]; then shift; exec "$@"; fi
    shift
  done
fi
`
	if err := os.WriteFile(filepath.Join(dir, "nix"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logFile
}

func rootsPlan(t *testing.T, env string) (*Plan, string) {
	t.Helper()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	return &Plan{
		Site: "s", View: "v",
		Vars:  Vars{Repo: repo, System: "/sys", Out: filepath.Join(dir, "out"), Cache: filepath.Join(dir, "cache", "s")},
		Steps: []Step{{Kind: "run", Run: "echo built > $DDM_OUT/x", Env: env, From: "w"}},
	}, dir
}

func nixCalls(t *testing.T, logFile string) []string {
	t.Helper()
	b, _ := os.ReadFile(logFile)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestStepEnvIsRooted(t *testing.T) {
	logFile := fakeNix(t)
	p, dir := rootsPlan(t, "/sys#mkdocs")
	if _, err := p.Execute(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "cache", "s", "gcroots", "system-mkdocs")
	calls := nixCalls(t, logFile)
	if want := "develop /sys#mkdocs --profile " + root + " --command sh -euc echo built > $DDM_OUT/x"; len(calls) != 1 || calls[0] != want {
		t.Errorf("nix calls:\n got %q\nwant [%q]", calls, want)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "out", "x")); string(got) != "built\n" {
		t.Errorf("the step did not run in the fake env: %q", got)
	}
	if fi, err := os.Stat(filepath.Dir(root)); err != nil || !fi.IsDir() {
		t.Errorf("the roots directory should exist: %v", err)
	}
}

// A changed environment adds a generation to the profile and the old one stays
// rooted, so superseded toolchains would be kept for ever; they are wiped, but
// only when there is something to wipe.
func TestOldGenerationsAreWiped(t *testing.T) {
	for _, c := range []struct {
		name  string
		links []string
		wipe  bool
	}{
		{"one generation", []string{"system-mkdocs-1-link"}, false},
		{"two generations", []string{"system-mkdocs-1-link", "system-mkdocs-2-link"}, true},
		{"another profile's generations do not count", []string{"system-xslt-1-link", "system-xslt-2-link", "system-mkdocs-1-link"}, false},
		{"not a generation", []string{"system-mkdocs-1-link", "system-mkdocs-notes-link"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			logFile := fakeNix(t)
			p, dir := rootsPlan(t, "/sys#mkdocs")
			roots := filepath.Join(dir, "cache", "s", "gcroots")
			os.MkdirAll(roots, 0o755)
			for _, l := range c.links {
				os.WriteFile(filepath.Join(roots, l), nil, 0o644)
			}
			if _, err := p.Execute(context.Background(), &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			wipe := "profile wipe-history --profile " + filepath.Join(roots, "system-mkdocs")
			var got bool
			for _, call := range nixCalls(t, logFile) {
				got = got || call == wipe
			}
			if got != c.wipe {
				t.Errorf("wipe-history called = %v, want %v (calls %q)", got, c.wipe, nixCalls(t, logFile))
			}
		})
	}
}

// The host environment needs no toolchain, and a plan with no cache has
// nowhere to put a root: both run as before.
func TestNoRootWithoutEnvOrCache(t *testing.T) {
	logFile := fakeNix(t)
	p, _ := rootsPlan(t, "")
	if _, err := p.Execute(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logFile); err == nil {
		t.Errorf("a step with no env should not call nix")
	}

	p, _ = rootsPlan(t, "/sys#mkdocs")
	p.Vars.Cache = ""
	if _, err := p.Execute(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if calls := nixCalls(t, logFile); len(calls) != 1 || strings.Contains(calls[0], "--profile") {
		t.Errorf("with no cache the env still runs, unrooted: %q", calls)
	}
}
