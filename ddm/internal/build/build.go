// Package build runs one view of one site end to end: find the commit, check
// it out, resolve and execute the workflow, and keep the result under
// <work>/out/<site>/<view>/<commit>/. Publishing that directory is M3's job.
// Only the newest build of a view is kept there (see keep).
package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ddm/internal/fsutil"
	"ddm/internal/gitsync"
	"ddm/internal/spec"
	"ddm/internal/workflow"
)

// Request describes one build.
type Request struct {
	Spec   *spec.Spec
	Site   string
	View   string
	System string // absolute path of the repo holding the spec
	Work   string // root for checkouts, scratch and results

	Src    string // optional local repo to build from instead of the site's repo
	Branch string // optional branch override

	Overrides map[string]string
	DryRun    bool
	Log       io.Writer
}

// Result is where a finished build ended up.
type Result struct {
	Commit string // the sha built, "none" for sites without a repo, or a pool's combined id
	Dir    string // <work>/out/<served>/<view>/<commit>
	Served string // what to publish it as: the site, or its pool (spec.Served)
	// Members is what was built: the site's repo, branch and commit, or every
	// member of a pool. The toolbar's status line shows it.
	Members []Member
}

// Run performs the build described by r.
func Run(ctx context.Context, r Request) (*Result, error) {
	site, ok := r.Spec.Sites[r.Site]
	if !ok {
		// Resolve gives the friendlier error, with the list of sites.
		_, err := workflow.Resolve(r.Spec, r.Site, r.View, workflow.Vars{}, nil)
		return nil, err
	}
	work, err := filepath.Abs(r.Work)
	if err != nil {
		return nil, err
	}
	if wf := r.Spec.PoolOf(r.Site, r.View); wf != "" {
		return runPool(ctx, r, work, wf)
	}
	checkout := filepath.Join(work, "src", r.Site)
	scratch := filepath.Join(work, "scratch", r.Site, r.View)

	commit := "none"
	if site.Repo != "" || r.Src != "" {
		url := gitsync.CloneURL(site.Repo)
		if r.Src != "" {
			if url, err = filepath.Abs(r.Src); err != nil {
				return nil, err
			}
		}
		branch := site.Branch
		if r.Branch != "" {
			branch = r.Branch
		}
		if commit, err = gitsync.RemoteHead(url, branch); err != nil {
			return nil, err
		}
		fmt.Fprintf(r.Log, "==> %s %s is at %s\n", url, branch, commit)
		if !r.DryRun {
			if err := gitsync.Sync(url, branch, commit, checkout); err != nil {
				return nil, err
			}
		}
	} else if err := os.MkdirAll(checkout, 0o755); err != nil {
		return nil, err
	}

	plan, err := workflow.Resolve(r.Spec, r.Site, r.View, workflow.Vars{
		Repo: checkout, System: r.System, Out: scratch, Cache: filepath.Join(work, "cache", r.Site),
	}, r.Overrides)
	if err != nil {
		return nil, err
	}
	if r.DryRun {
		fmt.Fprint(r.Log, plan)
		return &Result{Commit: commit}, nil
	}

	if err := os.RemoveAll(scratch); err != nil {
		return nil, err
	}
	built, err := plan.Execute(ctx, r.Log)
	if err != nil {
		return nil, err
	}

	dest, err := keep(built, work, r.Site, r.View, commit, r.Log)
	if err != nil {
		return nil, err
	}
	branch := site.Branch
	if r.Branch != "" {
		branch = r.Branch
	}
	m := Member{Site: r.Site}
	if commit != "none" {
		m.Repo, m.Branch, m.Commit = site.Repo, branch, commit
	}
	return &Result{Commit: commit, Dir: dest, Served: r.Site, Members: []Member{m}}, nil
}

// keep copies a build result to <work>/out/<served>/<view>/<commit> and then
// removes every other build of that view.
//
// out is the hand-off from a build to its publish. Publishing copies the
// build to the web root, which keeps the history (publish.DefaultKeep), and
// nothing reads an older build in out again: the build state records
// commits, not paths. Keeping them all only filled the disk (a forest build is
// about 250 MB; the droplet's 24 GB filled up on 2026-11-13 with 29 of them).
//
// Builds of one site and view do not overlap, since they share a scratch
// directory, so removing the others cannot pull a build from under a publish.
func keep(built, work, served, view, commit string, log io.Writer) (string, error) {
	root := filepath.Join(work, "out", served, view)
	dest := filepath.Join(root, commit)
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := fsutil.CopyTree(built, dest); err != nil {
		return "", fmt.Errorf("copying result: %w", err)
	}
	// A failure here must not fail a finished build, but it must be seen.
	if err := pruneBuilds(root, commit); err != nil {
		fmt.Fprintf(log, "    warning: removing older builds: %v\n", err)
	}
	return dest, nil
}

// pruneBuilds removes every build under root except live. Names starting
// with a dot, and anything that is not a directory, are left alone.
func pruneBuilds(root, live string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var errs []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == live || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// Head returns what building r now would build, without checking anything
// out: the branch's commit, "none" for a site without a repo, or a pool's id.
// Src and Branch overrides are ignored: it is for the server.
func Head(r Request) (string, error) {
	site, ok := r.Spec.Sites[r.Site]
	if !ok {
		return "", fmt.Errorf("no site %q", r.Site)
	}
	if wf := r.Spec.PoolOf(r.Site, r.View); wf != "" {
		var members []Member
		for _, name := range r.Spec.PoolMembers(wf, r.View) {
			m := Member{Site: name}
			if repo := r.Spec.Sites[name].Repo; repo != "" {
				commit, err := gitsync.RemoteHead(gitsync.CloneURL(repo), r.Spec.Sites[name].Branch)
				if err != nil {
					return "", fmt.Errorf("%s: %w", name, err)
				}
				m.Commit = commit
			}
			members = append(members, m)
		}
		return poolID(members), nil
	}
	if site.Repo == "" {
		return "none", nil
	}
	return gitsync.RemoteHead(gitsync.CloneURL(site.Repo), site.Branch)
}
