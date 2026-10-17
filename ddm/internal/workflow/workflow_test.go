package workflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ddm/internal/spec"
)

func mustSpec(t *testing.T, src string) *spec.Spec {
	t.Helper()
	s, err := spec.Parse([]byte(src))
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	return s
}

const specSrc = `
domains: [cathe.dev]
workflows:
  outer:
    env: { flake: "{{system}}#outer" }
    params: { greeting: hello, ml_dir: lib }
    steps:
      - run: echo {{greeting}} from {{site}}/{{view}} > {{out}}/a.txt
      - run: echo private only
        when: { view: private }
      - run: echo not private
        unless: { view: private }
      - use: inner
  inner:
    params: { greeting: overridden-by-caller, depth: inner }
    steps:
      - run: echo {{greeting}} {{depth}}
      - builtin: mkdocs-config
        params: { input_dir: "{{ml_dir}}" }
  plain:
    steps:
      - run: echo {{greeting}}
sites:
  s:
    views:
      public:  { workflow: outer }
      private: { workflow: outer, params: { greeting: hi } }
    mounts: [ { host: s.cathe.dev } ]
  typo:
    workflow: plain
    mounts: [ { host: typo.cathe.dev } ]
`

func runs(p *Plan) []string {
	var out []string
	for _, st := range p.Steps {
		if st.Kind == "run" {
			out = append(out, st.Run)
		}
	}
	return out
}

func TestResolve(t *testing.T) {
	s := mustSpec(t, specSrc)
	vars := Vars{Repo: "/r", System: "/sys", Out: "/o"}

	p, err := Resolve(s, "s", "public", vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"echo hello from s/public > /o/a.txt",
		"echo not private",
		"echo hello inner", // caller's greeting beats inner's default; inner's own default fills depth
	}
	if got := runs(p); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("public runs:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, st := range p.Steps {
		// inner has no env of its own, so it inherits outer's.
		if st.Env != "/sys#outer" {
			t.Errorf("step %+v: env %q, want /sys#outer", st, st.Env)
		}
	}
	last := p.Steps[len(p.Steps)-1]
	if last.Builtin != "mkdocs-config" || last.Params["input_dir"] != "lib" {
		t.Errorf("builtin params not substituted: %+v", last)
	}

	p, err = Resolve(s, "s", "private", vars, map[string]string{"greeting": "cli"})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"echo cli from s/private > /o/a.txt", "echo private only", "echo cli inner"}
	if got := runs(p); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("private runs with override:\n%s", strings.Join(got, "\n"))
	}
}

func TestResolveErrors(t *testing.T) {
	s := mustSpec(t, specSrc)
	vars := Vars{Repo: "/r", System: "/sys", Out: "/o"}
	if _, err := Resolve(s, "typo", "public", vars, nil); err == nil || !strings.Contains(err.Error(), "unknown parameter {{greeting}}") {
		t.Fatalf("want unknown-parameter error, got %v", err)
	}
	if _, err := Resolve(s, "nope", "public", vars, nil); err == nil || !strings.Contains(err.Error(), `no site "nope"`) {
		t.Fatalf("want no-site error, got %v", err)
	}
	if _, err := Resolve(s, "s", "notes", vars, nil); err == nil || !strings.Contains(err.Error(), `no view "notes"`) {
		t.Fatalf("want no-view error, got %v", err)
	}
}

// The repo's own workflows must all resolve for every view of every site.
func TestRepoSpecResolves(t *testing.T) {
	s, err := spec.Load("../../../sites.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range s.SiteNames() {
		for view := range s.Sites[name].Views {
			if _, err := Resolve(s, name, view, Vars{Repo: "/r", System: "/sys", Out: "/o"}, nil); err != nil {
				t.Errorf("%s/%s: %v", name, view, err)
			}
		}
	}
}

func TestBuiltinsCoverSpec(t *testing.T) {
	for name := range spec.Builtins {
		if _, ok := Builtins[name]; !ok {
			t.Errorf("spec allows builtin %q but workflow has no implementation entry", name)
		}
	}
	for name := range Builtins {
		if !spec.Builtins[name] {
			t.Errorf("builtin %q is implemented but the spec rejects it", name)
		}
	}
}

func TestExecute(t *testing.T) {
	repo, out := t.TempDir(), filepath.Join(t.TempDir(), "out")
	if err := os.Mkdir(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &Plan{Site: "s", View: "public", Vars: Vars{Repo: repo, Out: out}, Steps: []Step{
		{Kind: "builtin", Builtin: "mkdocs-config", Params: map[string]string{"input_dir": "docs"}, From: "t"},
		{Kind: "run", Run: `mkdir -p site && echo "$DDM_SITE/$DDM_VIEW" > site/index.html`, From: "t"},
		{Kind: "output", Output: "site", From: "t"},
	}}
	var log bytes.Buffer
	res, err := p.Execute(context.Background(), &log)
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	if res != filepath.Join(repo, "site") {
		t.Errorf("result %q", res)
	}
	if b, _ := os.ReadFile(filepath.Join(res, "index.html")); string(b) != "s/public\n" {
		t.Errorf("index.html = %q (env vars not passed?)", b)
	}
	if b, err := os.ReadFile(filepath.Join(repo, "mkdocs.yml")); err != nil || !strings.Contains(string(b), `docs_dir: "docs"`) {
		t.Errorf("mkdocs.yml not written: %v %q", err, b)
	}
}

func TestExecuteFailures(t *testing.T) {
	cases := map[string][]Step{
		"failing command":  {{Kind: "run", Run: "exit 3"}},
		"escaping output":  {{Kind: "output", Output: "../elsewhere"}},
		"empty result":     {{Kind: "run", Run: "true"}},
		"unfinished build": {{Kind: "builtin", Builtin: "md-ssg"}},
		"mkdocs root dir":  {{Kind: "builtin", Builtin: "mkdocs-config", Params: map[string]string{"input_dir": "./"}}},
	}
	for name, steps := range cases {
		t.Run(name, func(t *testing.T) {
			p := &Plan{Site: "s", View: "public", Vars: Vars{Repo: t.TempDir(), Out: filepath.Join(t.TempDir(), "out")}, Steps: steps}
			if _, err := p.Execute(context.Background(), &bytes.Buffer{}); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
