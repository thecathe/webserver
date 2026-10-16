package spec

import (
	"strings"
	"testing"
	"time"
)

// base is a small valid spec; cases below each break one thing.
const base = `
domains: [cathe.dev, jpears.me]
taxa: [docs, notes]
workflows:
  forester:
    env: { flake: "{{repo}}" }
    steps:
      - run: forester build
      - builtin: rewrite-links
        unless: { view: private }
      - output: output/
  mkdocs:
    steps:
      - builtin: mkdocs-config
      - run: mkdocs build -d {{out}}
sites:
  forest:
    repo: thecathe/forest
    views:
      public:  { workflow: forester }
      private: { workflow: forester, params: { view: private } }
    mounts:
      - { host: jpears.me, entry: /@jpears/ }
      - { host: the.cathe.dev, view: private, access: tailnet-owner }
  docs:
    repo: thecathe/docs
    workflow: mkdocs
    taxa: [docs]
    mounts: [ { host: docs.cathe.dev, path: guide/ } ]
`

func TestValid(t *testing.T) {
	s, err := Parse([]byte(base))
	if err != nil {
		t.Fatalf("base spec should be valid: %v", err)
	}

	docs := s.Sites["docs"]
	if _, ok := docs.Views[DefaultView]; !ok || docs.Workflow != "" {
		t.Errorf("workflow shorthand should become a single public view, got %+v", docs)
	}
	if docs.Branch != DefaultBranch {
		t.Errorf("branch default: got %q", docs.Branch)
	}
	if got := docs.Mounts[0].Path; got != "/guide" {
		t.Errorf("path normalisation: got %q", got)
	}
	m := s.Sites["forest"].Mounts[0]
	if m.View != DefaultView || m.Access != AccessPublic || m.Path != "/" {
		t.Errorf("mount defaults: got %+v", m)
	}
	if time.Duration(s.System.Poll) != 10*time.Minute {
		t.Errorf("poll default: got %s", s.System.Poll)
	}
	if !docs.IsIndexed() {
		t.Errorf("sites are indexed by default")
	}
}

// The repo's own spec must always be valid.
func TestRepoSpec(t *testing.T) {
	if _, err := Load("../../../sites.yaml"); err != nil {
		t.Fatalf("sites.yaml: %v", err)
	}
}

func TestInvalid(t *testing.T) {
	cases := []struct {
		name    string
		old     string // replaced in base
		new     string
		wantErr string
	}{
		{"undeclared domain", "host: docs.cathe.dev", "host: cv.cathe.work", `"cv.cathe.work" is not under any declared domain`},
		{"apex lookalike", "host: docs.cathe.dev", "host: notcathe.dev", `"notcathe.dev" is not under any declared domain`},
		{"unknown workflow", "workflow: mkdocs\n", "workflow: hugo\n", `unknown workflow "hugo"`},
		{"unknown view", "view: private, access", "view: secret, access", `view "secret" is not one of this site's views`},
		{"duplicate mount", "host: docs.cathe.dev, path: guide/", "host: jpears.me", "jpears.me/ is already mounted by sites.docs.mounts[0]"},
		{"unknown field", "repo: thecathe/docs", "repo: thecathe/docs\n    reop: typo", "field reop not found"},
		{"both forms", "views:\n      public:  { workflow: forester }", "workflow: forester\n    views:\n      public:  { workflow: forester }", "use either workflow"},
		{"undeclared taxon", "taxa: [docs]\n    mounts", "taxa: [blog]\n    mounts", `taxon "blog" is not declared`},
		{"two kinds in one step", "- run: forester build", "- { run: forester build, output: x }", "set exactly one of run, builtin, use, output"},
		{"unknown builtin", "builtin: mkdocs-config", "builtin: mkdocs-magic", `unknown builtin "mkdocs-magic"`},
		{"bad site name", "  docs:\n    repo", "  Docs_Site:\n    repo", "sites.Docs_Site: name must match"},
		{"bad entry", "entry: /@jpears/", "entry: \"@jpears/\"", "must start with /"},
		{"bad access", "access: tailnet-owner", "access: everyone", `access "everyone"`},
		{"no mounts", "    mounts: [ { host: docs.cathe.dev, path: guide/ } ]\n", "", "no mounts"},
		{"bad repo", "repo: thecathe/forest", "repo: forest", `repo "forest" must be owner/name`},
		{"use unknown", "- builtin: mkdocs-config", "- use: nope", `uses unknown workflow "nope"`},
		{"bad poll", "domains:", "system: { poll: soon }\ndomains:", "invalid duration"},
		{"flake without host", "domains:", "system: { flake: github:x/y }\ndomains:", "host is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(base, c.old) {
				t.Fatalf("test bug: %q not in base", c.old)
			}
			_, err := Parse([]byte(strings.Replace(base, c.old, c.new, 1)))
			if err == nil {
				t.Fatalf("want error containing %q, got none", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("want error containing %q, got:\n%v", c.wantErr, err)
			}
		})
	}
}

func TestUseCycle(t *testing.T) {
	src := `
domains: [cathe.dev]
workflows:
  a: { steps: [ { use: b } ] }
  b: { steps: [ { run: x }, { use: c } ] }
  c: { steps: [ { use: a } ] }
sites:
  s: { workflow: a, mounts: [ { host: s.cathe.dev } ] }
`
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "use cycle a → b → c → a") {
		t.Fatalf("want cycle a → b → c → a, got %v", err)
	}
}

// All problems are reported together, not just the first.
func TestReportsEveryProblem(t *testing.T) {
	_, err := Parse([]byte(`sites: { x: { workflow: nope } }`))
	if err == nil {
		t.Fatal("want errors")
	}
	for _, want := range []string{"at least one domain", `unknown workflow "nope"`, "no mounts"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}
