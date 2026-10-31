package gitsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneURL(t *testing.T) {
	cases := map[string]string{
		"thecathe/forest":                    "https://github.com/thecathe/forest.git",
		"https://example.org/x.git":          "https://example.org/x.git",
		"git@github.com:thecathe/forest.git": "git@github.com:thecathe/forest.git",
		"/srv/repos/x":                       "/srv/repos/x",
	}
	for in, want := range cases {
		if got := CloneURL(in); got != want {
			t.Errorf("CloneURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// origin builds a throwaway repo with two commits on main and returns its
// path and both shas.
func origin(t *testing.T) (dir, first, second string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "page.md"), "one")
	run("add", ".")
	run("commit", "-q", "-m", "one")
	first = run("rev-parse", "HEAD")
	write(t, filepath.Join(dir, "page.md"), "two")
	run("commit", "-q", "-am", "two")
	second = run("rev-parse", "HEAD")
	return dir, first, second
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSync(t *testing.T) {
	src, first, second := origin(t)

	head, err := RemoteHead(src, "main")
	if err != nil || head != second {
		t.Fatalf("RemoteHead = %q, %v; want %q", head, err, second)
	}

	dst := filepath.Join(t.TempDir(), "checkout")
	if err := Sync(src, "main", first, dst); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if got := read(t, filepath.Join(dst, "page.md")); got != "one" {
		t.Fatalf("after sync to first: page.md = %q", got)
	}

	// A previous build's leftovers must not survive the next sync.
	write(t, filepath.Join(dst, "output.html"), "stale")
	write(t, filepath.Join(dst, "page.md"), "edited")
	if err := Sync(src, "main", second, dst); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := read(t, filepath.Join(dst, "page.md")); got != "two" {
		t.Fatalf("after sync to second: page.md = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "output.html")); !os.IsNotExist(err) {
		t.Fatalf("untracked output.html survived the sync")
	}
}

func TestRemoteHeadMissingBranch(t *testing.T) {
	src, _, _ := origin(t)
	if _, err := RemoteHead(src, "nope"); err == nil {
		t.Fatal("want an error for a missing branch")
	}
}

// gitIn runs git in dir as a throwaway committer, with local-path submodules allowed.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// A repo with two submodules: "kept" is fetchable, "gone" names a repository that is not there (a pointer to a
// repo not yet published, as the forest's artifacts submodule is before its repo is pushed).
func withUnfetchableSubmodule(t *testing.T) (super, sha string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
	kept, _, tip := origin(t)
	super = t.TempDir()
	gitIn(t, super, "init", "-q", "-b", "main")
	write(t, filepath.Join(super, "page.md"), "super")
	gitIn(t, super, "submodule", "add", "-q", kept, "kept")
	gitIn(t, super, "config", "-f", ".gitmodules", "submodule.gone.path", "gone")
	gitIn(t, super, "config", "-f", ".gitmodules", "submodule.gone.url", "/nonexistent/gone")
	gitIn(t, super, "update-index", "--add", "--cacheinfo", "160000,"+tip+",gone")
	if err := os.MkdirAll(filepath.Join(super, "gone"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, super, "add", ".gitmodules")
	gitIn(t, super, "commit", "-q", "-m", "submodules")
	return super, gitIn(t, super, "rev-parse", "HEAD")
}

func TestSyncFailsOnASubmoduleThatCannotBeFetched(t *testing.T) {
	super, sha := withUnfetchableSubmodule(t)
	if err := Sync(super, "main", sha, filepath.Join(t.TempDir(), "checkout")); err == nil {
		t.Fatal("Sync succeeded with a submodule that cannot be fetched")
	}
}

func TestSyncSkipsTheSubmodulesItIsToldTo(t *testing.T) {
	super, sha := withUnfetchableSubmodule(t)
	dst := filepath.Join(t.TempDir(), "checkout")
	if err := Sync(super, "main", sha, dst, "gone"); err != nil {
		t.Fatalf("Sync with the unfetchable one skipped: %v", err)
	}
	if got := read(t, filepath.Join(dst, "kept", "page.md")); got != "two" {
		t.Fatalf("the submodule not skipped is populated: kept/page.md = %q", got)
	}
	if entries, err := os.ReadDir(filepath.Join(dst, "gone")); err != nil || len(entries) != 0 {
		t.Fatalf("the skipped submodule's directory should be empty: %v, %v", entries, err)
	}
	// The repo's own .gitmodules is untouched, so a person's `git submodule update --init` still means to get it.
	if got := read(t, filepath.Join(dst, ".gitmodules")); strings.Contains(got, "update") {
		t.Fatalf(".gitmodules was given an update setting:\n%s", got)
	}
}
