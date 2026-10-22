package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BuiltinArgs is what a builtin step receives.
type BuiltinArgs struct {
	Plan   *Plan
	Params map[string]string // workflow params in scope, plus the step's own
	Log    io.Writer
}

// BuiltinFunc implements a `builtin:` step.
type BuiltinFunc func(ctx context.Context, a BuiltinArgs) error

// Builtins must have an entry for every name in spec.Builtins (a test
// checks this), so a spec that validates never names a missing builtin.
var Builtins = map[string]BuiltinFunc{
	"mkdocs-config":  mkdocsConfig,
	"site-directory": siteDirectory,
	"md-ssg":         notYet("a later milestone"),
}

func notYet(when string) BuiltinFunc {
	return func(context.Context, BuiltinArgs) error {
		return fmt.Errorf("not implemented yet (planned for %s)", when)
	}
}

// mkdocsConfig writes a minimal mkdocs.yml when the repo has none, as the
// old demos-deploy-docs-md.yml Action did. Params: site_name (default: the
// site name), input_dir (docs_dir; default "docs"), theme (default
// "material"). When this repo has scripts/mkdocs_ddm_generated.py, the
// config also lists it as a hook: it turns a page's `generated` front matter
// into the tag the AI-generated marker reads (ddm/internal/aimark).
func mkdocsConfig(_ context.Context, a BuiltinArgs) error {
	repo := a.Plan.Vars.Repo
	for _, name := range []string{"mkdocs.yml", "mkdocs.yaml"} {
		if _, err := os.Stat(filepath.Join(repo, name)); err == nil {
			fmt.Fprintf(a.Log, "    %s exists; leaving it alone\n", name)
			return nil
		}
	}
	docsDir := strings.Trim(param(a.Params, "input_dir", "docs"), "/")
	if docsDir == "" || docsDir == "." {
		return errors.New("input_dir must name a subdirectory (mkdocs can't use the repo root as docs_dir)")
	}
	if _, err := os.Stat(filepath.Join(repo, docsDir)); err != nil {
		return fmt.Errorf("input_dir %q: %w", docsDir, err)
	}
	cfg := fmt.Sprintf("# Written by ddm's mkdocs-config builtin: the repo has no mkdocs.yml.\nsite_name: %q\ndocs_dir: %q\ntheme:\n  name: %q\n",
		param(a.Params, "site_name", a.Plan.Site), docsDir, param(a.Params, "theme", "material"))
	hook := filepath.Join(a.Plan.Vars.System, "scripts", "mkdocs_ddm_generated.py")
	if _, err := os.Stat(hook); err == nil {
		cfg += fmt.Sprintf("hooks:\n  - %q\n", hook)
	}
	fmt.Fprintf(a.Log, "    no mkdocs.yml; writing a minimal one (docs_dir %s)\n", docsDir)
	return os.WriteFile(filepath.Join(repo, "mkdocs.yml"), []byte(cfg), 0o644)
}

func param(p map[string]string, k, def string) string {
	if v := p[k]; v != "" {
		return v
	}
	return def
}
