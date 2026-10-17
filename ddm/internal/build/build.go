// Package build runs one view of one site end to end: find the commit, check
// it out, resolve and execute the workflow, and keep the result under
// <work>/out/<site>/<view>/<commit>/. Publishing that directory is M3's job.
package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	Commit string // the sha built, or "none" for sites without a repo
	Dir    string // <work>/out/<site>/<view>/<commit>
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
		Repo: checkout, System: r.System, Out: scratch,
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

	dest := filepath.Join(work, "out", r.Site, r.View, commit)
	if err := os.RemoveAll(dest); err != nil {
		return nil, err
	}
	if err := fsutil.CopyTree(built, dest); err != nil {
		return nil, fmt.Errorf("copying result: %w", err)
	}
	return &Result{Commit: commit, Dir: dest}, nil
}
