// Package workflow turns a site view into a concrete list of build steps
// (Resolve) and runs them (Plan.Execute).
//
// Resolving happens entirely up front: params are substituted, `use:` steps
// are inlined, and when/unless conditions are decided. A typo'd {{param}}
// therefore fails before anything runs, and the plan can be printed for
// inspection (`ddm build --dry-run`).
package workflow

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"ddm/internal/spec"
)

// Vars are the built-in variables every workflow can use.
type Vars struct {
	Repo   string // absolute path of the site checkout
	System string // absolute path of this repo (the one holding sites.yaml)
	Out    string // absolute path of the scratch output directory
	Cache  string // absolute path of a directory kept between builds (e.g. an opam switch)
	Site   string
	View   string
}

// Plan is a fully resolved build: nothing in it refers to params any more.
type Plan struct {
	Site, View string
	Vars       Vars
	Steps      []Step
	Spec       *spec.Spec // for builtins that read it, e.g. site-directory
}

// Step is one resolved step.
type Step struct {
	Kind    string            // run, builtin or output
	Run     string            // shell command, for run
	Builtin string            // builtin name, for builtin
	Params  map[string]string // builtin params, and the workflow params in scope
	Output  string            // output path relative to the checkout, for output
	Env     string            // flake ref for `nix develop`; "" means the host environment
	From    string            // workflow the step came from, for logs
}

var varRE = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_-]*)\s*\}\}`)

// Resolve builds the plan for one view of one site. overrides (e.g. from
// `ddm build --set k=v`) beat everything else.
func Resolve(s *spec.Spec, site, view string, vars Vars, overrides map[string]string) (*Plan, error) {
	st, ok := s.Sites[site]
	if !ok {
		return nil, fmt.Errorf("no site %q (sites: %s)", site, strings.Join(s.SiteNames(), ", "))
	}
	v, ok := st.Views[view]
	if !ok {
		return nil, fmt.Errorf("site %q has no view %q (views: %s)", site, view, strings.Join(sortedKeys(st.Views), ", "))
	}
	vars.Site, vars.View = site, view

	// Precedence, lowest first: workflow defaults, built-in vars, the view's
	// params, overrides. Built-ins sit above defaults so a workflow default
	// like `view: public` can never misreport the view being built.
	params := map[string]string{}
	maps.Copy(params, s.Workflows[v.Workflow].Params)
	maps.Copy(params, vars.asParams())
	maps.Copy(params, v.Params)
	maps.Copy(params, overrides)

	p := &Plan{Site: site, View: view, Vars: vars, Spec: s}
	r := resolver{spec: s, plan: p}
	r.inline(v.Workflow, params, "")
	if err := errors.Join(r.errs...); err != nil {
		return nil, err
	}
	return p, nil
}

// ResolvePool builds the plan for a pooled workflow: one run for every
// member site. Only the workflow's own params apply (members' view params
// can't all win), plus overrides. {{site}} is the pool's name, and {{repo}}
// the workspace holding every member's checkout.
func ResolvePool(s *spec.Spec, wf, view string, vars Vars, overrides map[string]string) (*Plan, error) {
	if !s.Workflows[wf].Pool {
		return nil, fmt.Errorf("workflow %q is not pooled", wf)
	}
	vars.Site, vars.View = spec.PoolName(wf), view
	params := map[string]string{}
	maps.Copy(params, s.Workflows[wf].Params)
	maps.Copy(params, vars.asParams())
	maps.Copy(params, overrides)
	p := &Plan{Site: vars.Site, View: view, Vars: vars, Spec: s}
	r := resolver{spec: s, plan: p}
	r.inline(wf, params, "")
	if err := errors.Join(r.errs...); err != nil {
		return nil, err
	}
	return p, nil
}

func (v Vars) asParams() map[string]string {
	return map[string]string{"repo": v.Repo, "system": v.System, "out": v.Out, "cache": v.Cache, "site": v.Site, "view": v.View}
}

type resolver struct {
	spec *spec.Spec
	plan *Plan
	errs []error
}

// inline appends workflow name's steps to the plan. Its params fill in only
// what the caller hasn't already set, and its env (if any) replaces the
// caller's for its own steps. spec validation has already ruled out cycles.
func (r *resolver) inline(name string, callerParams map[string]string, callerEnv string) {
	wf := r.spec.Workflows[name]
	params := maps.Clone(wf.Params)
	if params == nil {
		params = map[string]string{}
	}
	maps.Copy(params, callerParams)

	env := callerEnv
	if wf.Env.Flake != "" {
		env = r.sub(name, "env.flake", wf.Env.Flake, params)
	}

	for i, st := range wf.Steps {
		where := fmt.Sprintf("steps[%d]", i)
		if !matches(st.When, params) || (len(st.Unless) > 0 && matches(st.Unless, params)) {
			continue
		}
		out := Step{Kind: st.Kind(), Env: env, From: name, Params: params}
		switch out.Kind {
		case "run":
			out.Run = r.sub(name, where+".run", st.Run, params)
		case "builtin":
			out.Builtin = st.Builtin
			bp := maps.Clone(params)
			for k, v := range st.Params {
				bp[k] = r.sub(name, where+".params."+k, v, params)
			}
			out.Params = bp
		case "output":
			out.Output = r.sub(name, where+".output", st.Output, params)
		case "use":
			r.inline(st.Use, params, env)
			continue
		}
		r.plan.Steps = append(r.plan.Steps, out)
	}
}

// matches reports whether every key in cond has that value in params. An
// empty cond matches, so a step without `when` always runs.
func matches(cond, params map[string]string) bool {
	for k, v := range cond {
		if params[k] != v {
			return false
		}
	}
	return true
}

// sub replaces every {{name}} in s, recording an error for unknown names.
func (r *resolver) sub(workflow, where, s string, params map[string]string) string {
	return varRE.ReplaceAllStringFunc(s, func(m string) string {
		name := varRE.FindStringSubmatch(m)[1]
		v, ok := params[name]
		if !ok {
			r.errs = append(r.errs, fmt.Errorf("workflows.%s.%s: unknown parameter {{%s}} (known: %s)",
				workflow, where, name, strings.Join(sortedKeys(params), ", ")))
			return m
		}
		return v
	})
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

// String renders the plan for --dry-run.
func (p *Plan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "site %s, view %s\n  repo   %s\n  system %s\n  out    %s\n", p.Site, p.View, p.Vars.Repo, p.Vars.System, p.Vars.Out)
	for i, st := range p.Steps {
		env := st.Env
		if env == "" {
			env = "host"
		}
		var what string
		switch st.Kind {
		case "run":
			what = st.Run
		case "builtin":
			what = st.Builtin
		case "output":
			what = st.Output
		}
		fmt.Fprintf(&b, "  %d. [%s] %-7s %s   (env: %s)\n", i+1, st.From, st.Kind, what, env)
	}
	return b.String()
}
