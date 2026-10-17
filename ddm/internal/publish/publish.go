// Package publish puts a finished build where the web server reads it:
//
//	<www>/<view>/<site>/<stamp>-<commit>/   one directory per publish
//	<www>/<view>/<site>/current             symlink to the live one
//
// A publish copies the build, resolves its site:// links for the target,
// then swaps `current` with a rename, which is atomic: a request sees either
// the old build or the new one, never a half-copied directory. Nothing live
// is ever deleted in place. Rolling back is pointing `current` back.
package publish

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"ddm/internal/fsutil"
	"ddm/internal/links"
)

// DefaultKeep is how many publishes of a site and view are kept.
const DefaultKeep = 3

// Request describes one publish.
type Request struct {
	Site, View string
	Commit     string // the commit built, or "none"
	From       string // the build result, e.g. .ddm/out/<site>/<view>/<commit>
	WWW        string // the publish root
	Links      links.Resolver
	// Strict fails the publish on an unresolved site:// link. Otherwise the
	// link is left as it is and reported as a warning.
	Strict bool
	Keep   int // publishes to keep; 0 means DefaultKeep
	Log    io.Writer
	Now    func() time.Time // for tests; nil means time.Now
}

// Root is where every publish of a site's view lives.
func Root(www, view, site string) string { return filepath.Join(www, view, site) }

// Current is the path the web server serves a site's view from.
func Current(www, view, site string) string { return filepath.Join(Root(www, view, site), "current") }

// Run publishes r.From and returns the new publish directory.
func Run(r Request) (string, error) {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	root := Root(r.WWW, r.View, r.Site)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	commit := r.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	// A timestamp first, so names sort by age and republishing the same
	// commit (e.g. after a spec change alters its links) gets a fresh name.
	name := now().UTC().Format("20060102T150405.000") + "-" + commit
	tmp := filepath.Join(root, "."+name+".tmp")
	dest := filepath.Join(root, name)

	if err := os.RemoveAll(tmp); err != nil {
		return "", err
	}
	if err := fsutil.CopyTree(r.From, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("copying build: %w", err)
	}
	n, err := r.Links.Rewrite(tmp, r.Site, r.View)
	fmt.Fprintf(r.Log, "    rewrote %d site:// link(s)\n", n)
	if err != nil {
		if r.Strict {
			os.RemoveAll(tmp)
			return "", err
		}
		fmt.Fprintf(r.Log, "    warning: %v\n", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	if err := swap(root, name); err != nil {
		return "", err
	}
	keep := r.Keep
	if keep <= 0 {
		keep = DefaultKeep
	}
	if err := prune(root, name, keep); err != nil {
		fmt.Fprintf(r.Log, "    warning: pruning old publishes: %v\n", err)
	}
	return dest, nil
}

// swap points root/current at name, atomically.
func swap(root, name string) error {
	tmp := filepath.Join(root, ".current.tmp")
	os.Remove(tmp)
	if err := os.Symlink(name, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, "current"))
}

// prune removes all but the newest keep publishes. live is never removed.
func prune(root, live string, keep int) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	var errs []string
	for _, n := range names[:max(0, len(names)-keep)] {
		if n == live {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, n)); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
