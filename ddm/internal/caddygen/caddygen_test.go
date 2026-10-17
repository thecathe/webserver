package caddygen

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"ddm/internal/spec"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const specSrc = `
domains: [cathe.dev, jpears.me]
private_domain: the.cathe.dev
workflows:
  w: { steps: [ { run: x } ] }
  dir: { steps: [ { builtin: site-directory } ] }
  pooled: { pool: true, steps: [ { run: x } ] }
sites:
  forest:
    views: { public: { workflow: w }, private: { workflow: w } }
    mounts:
      - { host: forest.cathe.dev, entry: /index/ }
      - { host: jpears.me, entry: /@jpears/ }
      - { host: the.cathe.dev, view: private, entry: /index/, access: private }
  docs:
    views: { public: { workflow: dir }, private: { workflow: dir } }
    mounts:
      - { host: docs.cathe.dev }
      - { host: the.cathe.dev, path: sites, view: private, access: private }
  guide:
    workflow: w
    mounts: [ { host: docs.cathe.dev, path: guide, entry: /intro/ } ]
  ocaml:
    workflow: pooled
    mounts: [ { host: ocaml.cathe.dev } ]
  lib:
    repo: me/lib
    workflow: pooled
    mounts: [ { host: lib.cathe.dev, entry: /lib/ } ]
  quiet:
    repo: me/quiet
    workflow: pooled
  deep:
    workflow: w
    mounts: [ { host: a.b.cathe.dev } ]
  secret:
    workflow: w
    access: private
    mounts: [ { host: secret.the.cathe.dev } ]
`

func TestGolden(t *testing.T) {
	s, err := spec.Parse([]byte(specSrc))
	if err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]Target{
		"prod.Caddyfile":    {WWW: "/var/www"},
		"preview.Caddyfile": {Preview: true, Port: 8080, WWW: "/home/me/.ddm/www"},
	} {
		t.Run(name, func(t *testing.T) {
			got := Generate(s, target)
			path := filepath.Join("testdata", name)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("%s differs from the golden file (go test ./internal/caddygen -update to accept):\n%s", name, got)
			}
			// The real Caddy must accept it (the prod config needs the
			// Cloudflare plugin: this repo's devShell has it).
			if _, err := exec.LookPath("caddy"); err != nil {
				t.Skip("caddy not on PATH")
			}
			out, err := exec.Command("caddy", "adapt", "--adapter", "caddyfile", "--config", path).CombinedOutput()
			if err != nil {
				t.Errorf("caddy adapt %s: %v\n%s", name, err, out)
			}
		})
	}
}
