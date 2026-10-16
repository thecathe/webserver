// Package gitsync finds the current commit of a site repo's branch and checks
// that exact commit out into a working directory.
//
// Builds always run against a specific sha rather than "whatever the branch
// is now", so a build, its output directory and its log all name the same
// commit.
package gitsync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneURL turns a spec repo into something git can clone: "owner/name" is
// GitHub over HTTPS (the server authenticates with one read-only token via a
// credential helper; locally your own git credentials apply). Anything else
// is passed through as a URL or path.
func CloneURL(repo string) string {
	if strings.Contains(repo, "://") || strings.HasPrefix(repo, "git@") || filepath.IsAbs(repo) {
		return repo
	}
	return "https://github.com/" + repo + ".git"
}

// RemoteHead returns the sha that branch points to on url, without cloning.
func RemoteHead(url, branch string) (string, error) {
	out, err := git("", "ls-remote", "--exit-code", url, "refs/heads/"+branch)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("git ls-remote %s %s: no output", url, branch)
	}
	return fields[0], nil
}

// Sync makes dir an exact, clean checkout of sha from url's branch. It clones
// on first use, otherwise fetches. Untracked and ignored files are removed,
// so leftovers from a previous build can't leak into the next one.
func Sync(url, branch, sha, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if _, err := git("", "clone", "--no-checkout", "--branch", branch, url, dir); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if _, err := git(dir, "remote", "set-url", "origin", url); err != nil {
			return err
		}
		if _, err := git(dir, "fetch", "--prune", "origin", branch); err != nil {
			return err
		}
	}
	if _, err := git(dir, "checkout", "--force", "--detach", sha); err != nil {
		return err
	}
	if _, err := git(dir, "submodule", "update", "--init", "--recursive", "--force"); err != nil {
		return err
	}
	_, err := git(dir, "clean", "-ffdx")
	return err
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Never prompt for credentials: a daemon has no one to answer.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
