// Package spec loads and validates sites.yaml: the server's system config
// source, the domains ddm may publish under, reusable build workflows, and the
// sites built from them. See docs/usage/spec.md for the user-facing reference.
package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Spec is the root of sites.yaml.
type Spec struct {
	System  System   `yaml:"system"`
	Paths   Paths    `yaml:"paths"`
	Domains []string `yaml:"domains"`
	// PrivateDomain is where private mounts live, e.g. "the.cathe.dev". It and
	// its subdomains resolve to the server's tailnet address, so only private
	// mounts may use it, and private mounts may use nothing else.
	PrivateDomain string   `yaml:"private_domain"`
	Taxa          []string `yaml:"taxa"`
	// Nav is the toolbar's links, shown on every site, in order.
	Nav       []NavLink           `yaml:"nav"`
	Workflows map[string]Workflow `yaml:"workflows"`
	Sites     map[string]Site     `yaml:"sites"`
}

// System says where the server's own NixOS config comes from.
type System struct {
	// Flake is the flake ddm rebuilds the server from, e.g.
	// "github:thecathe/webserver". ddm always pins the sha it saw.
	Flake string `yaml:"flake"`
	// Host selects nixosConfigurations.<host>.
	Host string `yaml:"host"`
	// Poll is how often this repo and every site repo are checked.
	Poll Duration `yaml:"poll"`
	// Cooldown is the default for every site's cooldown.
	Cooldown Duration `yaml:"cooldown"`
}

// Paths are where ddm keeps things on the machine it runs on.
type Paths struct {
	// Work holds site checkouts and scratch build output.
	Work string `yaml:"work"`
	// WWW is the publish root: <www>/<view>/<site>/<sha>/, plus current.
	WWW string `yaml:"www"`
	// Caddyfile is where the generated Caddy config is written.
	Caddyfile string `yaml:"caddyfile"`
}

// NavLink is one toolbar link. Link is site://name/path, mount://host/path
// or an https:// URL. A link to a private mount only shows on private pages.
type NavLink struct {
	Title string `yaml:"title"`
	Link  string `yaml:"link"`
}

// Workflow is a named, reusable list of build steps.
type Workflow struct {
	Env    Env               `yaml:"env"`
	Params map[string]string `yaml:"params"`
	Steps  []Step            `yaml:"steps"`
	// RootOnly marks workflows whose output uses root-absolute links (e.g.
	// forester's base-url="/"), so they can only be mounted at path "/".
	RootOnly bool `yaml:"root_only"`
	// Pool builds every site that uses this workflow (in the same view) together,
	// in one run over all of their checkouts, and serves the one result from
	// all of their mounts. Used for OCaml docs: odoc only links packages that
	// are built together.
	Pool bool `yaml:"pool"`
}

// Env names where a workflow's tools come from. Steps run inside
// `nix develop <flake> -c`. "{{repo}}" is the site's own checkout and
// "{{system}}#x" a devShell of this repo.
type Env struct {
	Flake string `yaml:"flake"`
}

// Step is exactly one of Run, Builtin, Use or Output.
type Step struct {
	Run     string `yaml:"run"`
	Builtin string `yaml:"builtin"`
	Use     string `yaml:"use"`
	Output  string `yaml:"output"`
	// Params for a builtin step.
	Params map[string]string `yaml:"params"`
	// When/Unless gate the step on parameter values, e.g. {view: private}.
	When   map[string]string `yaml:"when"`
	Unless map[string]string `yaml:"unless"`
}

// Kind reports which of the step fields is set, or "" if not exactly one.
func (s Step) Kind() string {
	var kinds []string
	if s.Run != "" {
		kinds = append(kinds, "run")
	}
	if s.Builtin != "" {
		kinds = append(kinds, "builtin")
	}
	if s.Use != "" {
		kinds = append(kinds, "use")
	}
	if s.Output != "" {
		kinds = append(kinds, "output")
	}
	if len(kinds) != 1 {
		return ""
	}
	return kinds[0]
}

// Site is one repo (or a generated site) built into one or more views and
// mounted on one or more hosts. Its map key in Spec.Sites is the name other
// sites use in site://<name>/<path> links.
type Site struct {
	// Repo is "owner/name" on GitHub, a full clone URL, or an absolute path. Empty for
	// generated sites such as the docs directory.
	Repo   string `yaml:"repo"`
	Branch string `yaml:"branch"`
	// Workflow and Params are shorthand for a single "public" view.
	Workflow string            `yaml:"workflow"`
	Params   map[string]string `yaml:"params"`
	Views    map[string]View   `yaml:"views"`
	Mounts   []Mount           `yaml:"mounts"`
	// Access is the default for every mount: "public" (default) or "private".
	Access string `yaml:"access"`

	Taxa        []string `yaml:"taxa"`
	Description string   `yaml:"description"`
	// Indexed lists the site in the docs.cathe.dev directory. Default true.
	Indexed *bool `yaml:"indexed"`
	// Cooldown is the least time between two builds of the site for new
	// commits: a busy repo is built at most this often, from its latest
	// commit. Default: system.cooldown.
	Cooldown *Duration `yaml:"cooldown"`
}

// IsIndexed reports whether the site appears in the site directory.
func (s Site) IsIndexed() bool { return s.Indexed == nil || *s.Indexed }

// View is one build of a site, e.g. public, notes or private.
type View struct {
	Workflow string            `yaml:"workflow"`
	Params   map[string]string `yaml:"params"`
}

// Mount serves one view of a site at host+path.
type Mount struct {
	Host string `yaml:"host"`
	// Path is a prefix under host. Default "/".
	Path string `yaml:"path"`
	// View defaults to "public".
	View string `yaml:"view"`
	// Entry is where requests for the mount's root redirect to, e.g. /@jpears/.
	Entry string `yaml:"entry"`
	// Access is "public" or "private" (served only on the tailnet, and only
	// to the owner's Tailscale identity). Defaults to the site's access.
	Access string `yaml:"access"`
}

const (
	DefaultView   = "public"
	DefaultBranch = "main"

	AccessPublic  = "public"
	AccessPrivate = "private"
)

// Builtins are the step names ddm implements in Go. Kept here so the spec can
// be validated before the workflow runner exists.
var Builtins = map[string]bool{
	"mkdocs-config":  true,
	"site-directory": true,
	"md-ssg":         true,
}

// Duration is a time.Duration written as "10m", "1h30m" etc., or a whole
// number of days or weeks: "1d", "2w". "0" is none.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	v, err := ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*d = Duration(v)
	return nil
}

// ParseDuration is time.ParseDuration, plus whole days ("1d") and weeks ("2w").
func ParseDuration(s string) (time.Duration, error) {
	units := map[byte]time.Duration{'d': 24 * time.Hour, 'w': 7 * 24 * time.Hour}
	if len(s) > 1 {
		if unit, ok := units[s[len(s)-1]]; ok {
			n, err := strconv.Atoi(s[:len(s)-1])
			if err != nil || n < 0 {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			return time.Duration(n) * unit, nil
		}
	}
	return time.ParseDuration(s)
}

// Cooldown is the least time between builds of a site's view for new
// commits. A pool's is the shortest of its members'.
func (s *Spec) Cooldown(site, view string) time.Duration {
	of := func(name string) time.Duration {
		if c := s.Sites[name].Cooldown; c != nil {
			return time.Duration(*c)
		}
		return time.Duration(s.System.Cooldown)
	}
	wf := s.PoolOf(site, view)
	if wf == "" {
		return of(site)
	}
	least := time.Duration(-1)
	for _, m := range s.PoolMembers(wf, view) {
		if c := of(m); least < 0 || c < least {
			least = c
		}
	}
	return max(least, 0)
}

func (d Duration) String() string { return time.Duration(d).String() }

// Load reads, normalises and validates the spec at path. Every problem found
// is reported at once rather than stopping at the first.
func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return Parse(data)
}

// Parse is Load without the file read.
func Parse(data []byte) (*Spec, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	// Unknown keys are almost always typos; fail loudly instead of ignoring them.
	dec.KnownFields(true)
	var s Spec
	// An empty document is let through so validate() can say what is missing.
	if err := dec.Decode(&s); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing: %w", err)
	}
	s.normalise()
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// normalise fills defaults so later code never has to.
func (s *Spec) normalise() {
	if s.System.Poll == 0 {
		s.System.Poll = Duration(10 * time.Minute)
	}
	if s.Paths.Work == "" {
		s.Paths.Work = "/var/lib/ddm/work"
	}
	if s.Paths.WWW == "" {
		s.Paths.WWW = "/var/www"
	}
	if s.Paths.Caddyfile == "" {
		s.Paths.Caddyfile = "/var/lib/ddm/Caddyfile"
	}
	s.PrivateDomain = strings.ToLower(s.PrivateDomain)
	for name, site := range s.Sites {
		if site.Branch == "" {
			site.Branch = DefaultBranch
		}
		// The shorthand becomes a single public view; validate() rejects
		// specs that use both forms.
		if site.Workflow != "" && len(site.Views) == 0 {
			site.Views = map[string]View{DefaultView: {Workflow: site.Workflow, Params: site.Params}}
			site.Workflow, site.Params = "", nil
		}
		if site.Access == "" {
			site.Access = AccessPublic
		}
		for i := range site.Mounts {
			m := &site.Mounts[i]
			m.Host = strings.ToLower(m.Host)
			if m.View == "" {
				m.View = DefaultView
			}
			if m.Access == "" {
				m.Access = site.Access
			}
			m.Path = normalisePath(m.Path)
		}
		s.Sites[name] = site
	}
}

// normalisePath makes "", "docs", "/docs/" all into "/" or "/docs".
func normalisePath(p string) string {
	p = "/" + strings.Trim(p, "/")
	return p
}

// PoolOf returns the pooled workflow a site's view is built by, or "".
func (s *Spec) PoolOf(site, view string) string {
	v, ok := s.Sites[site].Views[view]
	if ok && s.Workflows[v.Workflow].Pool {
		return v.Workflow
	}
	return ""
}

// PoolMembers returns the sites whose view is built by pooled workflow wf,
// sorted.
func (s *Spec) PoolMembers(wf, view string) []string {
	var out []string
	for _, name := range s.SiteNames() {
		if s.PoolOf(name, view) == wf {
			out = append(out, name)
		}
	}
	return out
}

// PoolName is the name a pool's build is published under. The "@" keeps it
// apart from site names.
func PoolName(wf string) string { return "@" + wf }

// Served returns the name a site's view is published under: its own name,
// or its pool's.
func (s *Spec) Served(site, view string) string {
	if wf := s.PoolOf(site, view); wf != "" {
		return PoolName(wf)
	}
	return site
}

// MountsServing returns every mount that serves what is published as served
// in view: one site's mounts, or all of a pool's members' mounts.
func (s *Spec) MountsServing(served, view string) []Mount {
	var out []Mount
	for _, name := range s.SiteNames() {
		if s.Served(name, view) != served {
			continue
		}
		for _, m := range s.Sites[name].Mounts {
			if m.View == view {
				out = append(out, m)
			}
		}
	}
	return out
}

// SiteNames returns the site names in sorted order.
func (s *Spec) SiteNames() []string { return sortedKeys(s.Sites) }
