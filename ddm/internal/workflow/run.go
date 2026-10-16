package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Execute runs the plan in p.Vars.Repo, writing progress and every step's
// output to log. It returns the absolute path of the build result: the last
// `output:` step's directory, or p.Vars.Out if there was none.
func (p *Plan) Execute(ctx context.Context, log io.Writer) (string, error) {
	if err := os.MkdirAll(p.Vars.Out, 0o755); err != nil {
		return "", err
	}
	result := p.Vars.Out
	for i, st := range p.Steps {
		start := time.Now()
		fmt.Fprintf(log, "==> %d/%d [%s] %s\n", i+1, len(p.Steps), st.From, describe(st))
		var err error
		switch st.Kind {
		case "run":
			err = p.run(ctx, st, log)
		case "builtin":
			err = p.builtin(ctx, st, log)
		case "output":
			result, err = p.output(st)
		default:
			err = fmt.Errorf("unknown step kind %q", st.Kind)
		}
		if err != nil {
			return "", fmt.Errorf("step %d (%s): %w", i+1, describe(st), err)
		}
		fmt.Fprintf(log, "    done in %s\n", time.Since(start).Round(time.Millisecond))
	}
	if err := nonEmptyDir(result); err != nil {
		return "", fmt.Errorf("build result %s: %w", result, err)
	}
	return result, nil
}

func describe(st Step) string {
	switch st.Kind {
	case "run":
		return st.Run
	case "builtin":
		return "builtin " + st.Builtin
	case "output":
		return "output " + st.Output
	}
	return st.Kind
}

// run executes a shell command in the checkout, inside `nix develop <env>`
// when the step has an env.
func (p *Plan) run(ctx context.Context, st Step, log io.Writer) error {
	argv := []string{"sh", "-euc", st.Run}
	if st.Env != "" {
		argv = append([]string{"nix", "develop", st.Env, "--command"}, argv...)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = p.Vars.Repo
	cmd.Env = append(os.Environ(),
		"DDM_SITE="+p.Site, "DDM_VIEW="+p.View, "DDM_OUT="+p.Vars.Out,
		"DDM_REPO="+p.Vars.Repo, "DDM_SYSTEM="+p.Vars.System)
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}

func (p *Plan) builtin(ctx context.Context, st Step, log io.Writer) error {
	fn, ok := Builtins[st.Builtin]
	if !ok {
		return fmt.Errorf("builtin %q is not registered", st.Builtin)
	}
	return fn(ctx, BuiltinArgs{Plan: p, Params: st.Params, Log: log})
}

// output resolves an `output:` path, which must stay inside the checkout
// (or be the scratch output dir itself).
func (p *Plan) output(st Step) (string, error) {
	path := st.Output
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.Vars.Repo, path)
	}
	path = filepath.Clean(path)
	if path != filepath.Clean(p.Vars.Out) && !within(p.Vars.Repo, path) {
		return "", fmt.Errorf("output %q is outside the checkout", st.Output)
	}
	return path, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func nonEmptyDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("is empty")
	}
	return nil
}
