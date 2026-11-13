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
	"slices"
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
	Nav []NavLink `yaml:"nav"`
	// Robots names the crawlers that robots.txt treats specially (D50).
	Robots    Robots              `yaml:"robots"`
	Workflows map[string]Workflow `yaml:"workflows"`
	Sites     map[string]Site     `yaml:"sites"`
}

// Robots lists crawlers by user agent. Every other crawler, search engines
// included, follows the `User-agent: *` rules, set per mount by Search.
type Robots struct {
	// Training crawlers (GPTBot, CCBot, …) are refused everywhere.
	Training []string `yaml:"training"`
	// Assistants fetch a page for a user (ChatGPT-User, Claude-User, …):
	// allowed only on mounts whose site sets assistants: true.
	Assistants []string `yaml:"assistants"`
}

// What search engines may index on a mount.
const (
	SearchNone  = "none"  // nothing (the default)
	SearchEntry = "entry" // the mount's root and its entry page, and the assets they need
	SearchAll   = "all"   // everything
)

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
	// Search is the default for every public mount: none (default), entry
	// or all. Private mounts are never indexed.
	Search string `yaml:"search"`
	// Assistants lets robots.Assistants fetch every public mount.
	Assistants bool `yaml:"assistants"`
	// Generated marks the site's pages as written by an AI: the shared
	// marker is drawn on them (see Generated). Nil means no site-wide mark;
	// a page's own `generated` front matter can still mark it.
	Generated *Generated `yaml:"generated"`
}

// IsIndexed reports whether the site appears in the site directory.
func (s Site) IsIndexed() bool { return s.Indexed == nil || *s.Indexed }

// View is one build of a site, e.g. public, notes or private.
type View struct {
	Workflow string            `yaml:"workflow"`
	Params   map[string]string `yaml:"params"`
	// Keep is how many publishes of this view stay in the web root (the live
	// one and the newest before it). 0, the default, means the publisher's
	// default (three). A big view that needs little redundancy can say 1.
	Keep int `yaml:"keep"`
}

// Keep returns how many publishes of site's view to keep, 0 meaning the
// default. A site or view that is not in the spec (a pooled build is published
// under a name that is not) also gives 0.
func (s *Spec) Keep(site, view string) int {
	return s.Sites[site].Views[view].Keep
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
	// Search overrides the site's search for this mount. Always none for a
	// private mount.
	Search string `yaml:"search"`
	// Assistants is the site's assistants (set when the spec is parsed).
	Assistants bool `yaml:"-"`
	// Mirrors is the public host this mount is the private twin of. Set only
	// on the mounts ddm derives (see mirror), never written in sites.yaml.
	Mirrors string `yaml:"-"`
}

const (
	DefaultView   = "public"
	PrivateView   = "private"
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
		if site.Generated != nil && site.Generated.By == "" {
			site.Generated.By = DefaultGeneratedBy
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
			if m.Search == "" {
				m.Search = site.Search
			}
			if m.Search == "" || m.Access == AccessPrivate {
				m.Search = SearchNone
			}
			m.Assistants = site.Assistants && m.Access == AccessPublic
		}
		s.Sites[name] = site
	}
	s.mirror()
}

// mirror adds the private twin of every public mount: the same site, path
// and entry, on the host with the private domain's extra label inserted
// (docs.cathe.dev gets docs.the.cathe.dev, cathe.dev gets the.cathe.dev). A
// twin serves the site's private view if it has one, else the public mount's
// own view, so a site with nothing private to show is served as it is, behind
// the gate. A mount written at the twin's address replaces the derived one.
//
// Derived mounts come after the written ones. A view's first mount is its
// canonical address (site:// links), so a twin never takes that from a public
// mount of the same view.
func (s *Spec) mirror() {
	if s.mirrorBase() == "" {
		return
	}
	type key struct{ host, path string }
	for name, site := range s.Sites {
		have := map[key]bool{}
		for _, m := range site.Mounts {
			have[key{m.Host, m.Path}] = true
		}
		_, hasPrivate := site.Views[PrivateView]
		for _, m := range slices.Clone(site.Mounts) {
			twin := s.PrivateTwin(m.Host)
			if m.Access != AccessPublic || twin == "" || have[key{twin, m.Path}] {
				continue
			}
			view := m.View
			if hasPrivate {
				view = PrivateView
			}
			site.Mounts = append(site.Mounts, Mount{
				Host: twin, Path: m.Path, View: view, Entry: m.Entry,
				Access: AccessPrivate, Search: SearchNone, Mirrors: m.Host,
			})
		}
		s.Sites[name] = site
	}
}

// mirrorBase is the declared domain the private domain sits under
// ("cathe.dev" for "the.cathe.dev"), or "" if there is no private domain or no
// room for an extra label, in which case no mount has a twin.
func (s *Spec) mirrorBase() string {
	pd := s.PrivateDomain
	if pd == "" || !hostRE.MatchString(pd) {
		return ""
	}
	if base := s.DomainOf(pd); base != pd {
		return base
	}
	return ""
}

// PrivateTwin returns the private-domain host that mirrors public host h
// (docs.cathe.dev -> docs.the.cathe.dev), or "" if h has none: it is already
// private, or not under the domain the private domain sits beneath.
func (s *Spec) PrivateTwin(h string) string {
	base := s.mirrorBase()
	switch {
	case base == "" || !hostRE.MatchString(h) || s.IsPrivateHost(h):
		return ""
	case h == base:
		return s.PrivateDomain
	case strings.HasSuffix(h, "."+base):
		return strings.TrimSuffix(h, base) + s.PrivateDomain
	}
	return ""
}

// PublicTwin is PrivateTwin's inverse: the public host a private-domain host
// mirrors (docs.the.cathe.dev -> docs.cathe.dev), or "" if it mirrors none.
// Whether anything is mounted there is for the caller to find out.
func (s *Spec) PublicTwin(h string) string {
	base := s.mirrorBase()
	switch {
	case base == "":
		return ""
	case h == s.PrivateDomain:
		return base
	case strings.HasSuffix(h, "."+s.PrivateDomain):
		return strings.TrimSuffix(h, s.PrivateDomain) + base
	}
	return ""
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
