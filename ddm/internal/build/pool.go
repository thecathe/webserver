package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ddm/internal/fsutil"
	"ddm/internal/gitsync"
	"ddm/internal/spec"
	"ddm/internal/workflow"
)

// Member is one site's contribution to a pooled build. The list is written
// into the result as ddm-build.json, so a published pool says exactly what
// it was built from.
type Member struct {
	Site   string `json:"site"`
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit,omitempty"`
}

// workspaceFile makes the pool's directory one dune workspace, so a single
// `dune build @doc` sees every member project and odoc links across them.
// Harmless for non-dune pools.
const workspaceFile = "(lang dune 3.0)\n; Written by ddm: every member site's checkout is a subdirectory.\n"

// runPool builds every member of pooled workflow wf at once: each member's
// repo is synced as usual, copied into <work>/scratch/<pool>/<view>/ws/<site>/,
// and the workflow runs once with that workspace as {{repo}}. Members without
// a repo (an index site) contribute only their mounts.
func runPool(ctx context.Context, r Request, work, wf string) (*Result, error) {
	pool := spec.PoolName(wf)
	members := r.Spec.PoolMembers(wf, r.View)
	scratch := filepath.Join(work, "scratch", pool, r.View)
	ws := filepath.Join(scratch, "ws")
	fmt.Fprintf(r.Log, "==> %s pools %s (%s)\n", r.Site, pool, strings.Join(members, ", "))

	var built []Member
	for _, name := range members {
		site := r.Spec.Sites[name]
		url, branch := gitsync.CloneURL(site.Repo), site.Branch
		if name == r.Site && r.Src != "" {
			abs, err := filepath.Abs(r.Src)
			if err != nil {
				return nil, err
			}
			url = abs
		}
		if name == r.Site && r.Branch != "" {
			branch = r.Branch
		}
		if site.Repo == "" && !(name == r.Site && r.Src != "") {
			built = append(built, Member{Site: name})
			continue
		}
		commit, err := gitsync.RemoteHead(url, branch)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		fmt.Fprintf(r.Log, "==> %s: %s %s is at %s\n", name, url, branch, commit)
		built = append(built, Member{Site: name, Repo: site.Repo, Branch: branch, Commit: commit})
		if r.DryRun {
			continue
		}
		checkout := filepath.Join(work, "src", name)
		if err := gitsync.Sync(url, branch, commit, checkout); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	id := poolID(built)

	plan, err := workflow.ResolvePool(r.Spec, wf, r.View, workflow.Vars{
		Repo: ws, System: r.System, Out: filepath.Join(scratch, "out"), Cache: filepath.Join(work, "cache", pool),
	}, r.Overrides)
	if err != nil {
		return nil, err
	}
	if r.DryRun {
		fmt.Fprint(r.Log, plan)
		return &Result{Commit: id, Served: pool}, nil
	}

	if err := os.RemoveAll(scratch); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return nil, err
	}
	for _, m := range built {
		if m.Commit == "" {
			continue
		}
		// Copies, not symlinks, and without build state or local switches.
		skip := func(rel string) bool {
			switch filepath.Base(rel) {
			case ".git", "_build", "_opam":
				return true
			}
			return false
		}
		if err := fsutil.CopyTreeSkip(filepath.Join(work, "src", m.Site), filepath.Join(ws, m.Site), skip); err != nil {
			return nil, fmt.Errorf("%s: copying into the workspace: %w", m.Site, err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "dune-workspace"), []byte(workspaceFile), 0o644); err != nil {
		return nil, err
	}

	result, err := plan.Execute(ctx, r.Log)
	if err != nil {
		return nil, err
	}
	dest, err := keep(result, work, pool, r.View, id)
	if err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(map[string]any{"pool": pool, "id": id, "members": built}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dest, "ddm-build.json"), append(manifest, '\n'), 0o644); err != nil {
		return nil, err
	}
	return &Result{Commit: id, Dir: dest, Served: pool, Members: built}, nil
}

// poolID names a pooled build by every member's commit: any member moving
// gives a new id.
func poolID(members []Member) string {
	h := sha256.New()
	for _, m := range members {
		fmt.Fprintf(h, "%s=%s\n", m.Site, m.Commit)
	}
	return "pool-" + hex.EncodeToString(h.Sum(nil))[:12]
}
