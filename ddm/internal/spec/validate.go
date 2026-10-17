package spec

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// nameRE constrains site, view and workflow names: they appear in
// site:// links, directory names and systemd unit names.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// hostRE is a lowercase DNS name with at least one dot.
var hostRE = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)

// repoRE accepts "owner/name" (GitHub shorthand).
var repoRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// problems collects every validation error so the user sees them all at once.
type problems []error

func (p *problems) add(format string, args ...any) {
	*p = append(*p, fmt.Errorf(format, args...))
}

func (s *Spec) validate() error {
	var p problems
	s.validateSystem(&p)
	s.validateDomains(&p)
	s.validateWorkflows(&p)
	s.validateSites(&p)
	s.validateNav(&p)
	return errors.Join(p...)
}

func (s *Spec) validateSystem(p *problems) {
	if s.System.Flake != "" && s.System.Host == "" {
		p.add("system: host is required when flake is set")
	}
	if s.System.Poll < Duration(0) {
		p.add("system: poll must be positive")
	}
}

func (s *Spec) validateDomains(p *problems) {
	if len(s.Domains) == 0 {
		p.add("domains: at least one domain is required")
	}
	seen := map[string]bool{}
	for _, d := range s.Domains {
		if !hostRE.MatchString(d) {
			p.add("domains: %q is not a valid lowercase domain name", d)
		}
		if seen[d] {
			p.add("domains: %q listed twice", d)
		}
		seen[d] = true
	}
	if pd := s.PrivateDomain; pd != "" {
		if !hostRE.MatchString(pd) {
			p.add("private_domain: %q is not a valid lowercase domain name", pd)
		} else if s.DomainOf(pd) == "" {
			p.add("private_domain: %q is not under any declared domain (%s)", pd, strings.Join(s.Domains, ", "))
		}
	}
}

func (s *Spec) validateWorkflows(p *problems) {
	for _, name := range sortedKeys(s.Workflows) {
		wf := s.Workflows[name]
		where := fmt.Sprintf("workflows.%s", name)
		if !nameRE.MatchString(name) {
			p.add("%s: name must match %s", where, nameRE)
		}
		if len(wf.Steps) == 0 {
			p.add("%s: no steps", where)
		}
		for i, st := range wf.Steps {
			at := fmt.Sprintf("%s.steps[%d]", where, i)
			switch st.Kind() {
			case "":
				p.add("%s: set exactly one of run, builtin, use, output", at)
			case "builtin":
				if !Builtins[st.Builtin] {
					p.add("%s: unknown builtin %q (known: %s)", at, st.Builtin, strings.Join(sortedKeys(Builtins), ", "))
				}
			case "use":
				if _, ok := s.Workflows[st.Use]; !ok {
					p.add("%s: uses unknown workflow %q", at, st.Use)
				}
			}
			if len(st.Params) > 0 && st.Kind() != "builtin" {
				p.add("%s: params only apply to builtin steps", at)
			}
		}
	}
	for _, cycle := range s.useCycles() {
		p.add("workflows: use cycle %s", strings.Join(cycle, " → "))
	}
}

// useCycles returns each `use:` cycle once, as the path that closes it.
func (s *Spec) useCycles() [][]string {
	const (
		unvisited = iota
		active
		done
	)
	state := map[string]int{}
	var cycles [][]string
	var stack []string
	var visit func(string)
	visit = func(name string) {
		state[name] = active
		stack = append(stack, name)
		for _, st := range s.Workflows[name].Steps {
			if st.Use == "" {
				continue
			}
			if _, ok := s.Workflows[st.Use]; !ok {
				continue // reported separately
			}
			switch state[st.Use] {
			case active:
				i := slices.Index(stack, st.Use)
				cycles = append(cycles, append(slices.Clone(stack[i:]), st.Use))
			case unvisited:
				visit(st.Use)
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = done
	}
	for _, name := range sortedKeys(s.Workflows) {
		if state[name] == unvisited {
			visit(name)
		}
	}
	return cycles
}

func (s *Spec) validateSites(p *problems) {
	if len(s.Sites) == 0 {
		p.add("sites: no sites defined")
	}
	type mountKey struct{ host, path string }
	owner := map[mountKey]string{}

	for _, name := range sortedKeys(s.Sites) {
		site := s.Sites[name]
		where := fmt.Sprintf("sites.%s", name)
		if !nameRE.MatchString(name) {
			p.add("%s: name must match %s (it is used in site:// links and paths)", where, nameRE)
		}
		if site.Repo != "" && !repoRE.MatchString(site.Repo) && !strings.Contains(site.Repo, "://") && !strings.HasPrefix(site.Repo, "git@") && !filepath.IsAbs(site.Repo) {
			p.add("%s: repo %q must be owner/name, a clone URL or an absolute path", where, site.Repo)
		}
		if site.Workflow != "" {
			p.add("%s: use either workflow (one public view) or views, not both", where)
		}
		if len(site.Views) == 0 {
			p.add("%s: needs a workflow or at least one view", where)
		}
		for _, vname := range sortedKeys(site.Views) {
			v := site.Views[vname]
			if !nameRE.MatchString(vname) {
				p.add("%s.views.%s: name must match %s", where, vname, nameRE)
			}
			if v.Workflow == "" {
				p.add("%s.views.%s: workflow is required", where, vname)
			} else if _, ok := s.Workflows[v.Workflow]; !ok {
				p.add("%s.views.%s: unknown workflow %q", where, vname, v.Workflow)
			}
		}
		if !validAccess(site.Access) {
			p.add("%s: access %q must be %q or %q", where, site.Access, AccessPublic, AccessPrivate)
		}
		for _, t := range site.Taxa {
			if len(s.Taxa) > 0 && !slices.Contains(s.Taxa, t) {
				p.add("%s: taxon %q is not declared in taxa", where, t)
			}
		}
		if len(site.Mounts) == 0 && !s.allPooled(name) {
			p.add("%s: no mounts, so it would never be served", where)
		}
		for i, m := range site.Mounts {
			at := fmt.Sprintf("%s.mounts[%d]", where, i)
			if !hostRE.MatchString(m.Host) {
				p.add("%s: host %q is not a valid domain name", at, m.Host)
			} else if s.DomainOf(m.Host) == "" {
				p.add("%s: host %q is not under any declared domain (%s)", at, m.Host, strings.Join(s.Domains, ", "))
			}
			if _, ok := site.Views[m.View]; !ok && len(site.Views) > 0 {
				p.add("%s: view %q is not one of this site's views (%s)", at, m.View, strings.Join(sortedKeys(site.Views), ", "))
			}
			if m.Entry != "" && !strings.HasPrefix(m.Entry, "/") {
				p.add("%s: entry %q must start with /", at, m.Entry)
			}
			if !validAccess(m.Access) {
				p.add("%s: access %q must be %q or %q", at, m.Access, AccessPublic, AccessPrivate)
			} else if hostRE.MatchString(m.Host) {
				s.validateAccessHost(p, at, m)
			}
			if v, ok := site.Views[m.View]; ok && s.Workflows[v.Workflow].RootOnly && m.Path != "/" {
				p.add("%s: workflow %s is root_only (its links are root-absolute), so it can't be mounted at path %s", at, v.Workflow, m.Path)
			}
			// One host+path serves exactly one thing, whatever its view.
			k := mountKey{m.Host, m.Path}
			if prev, dup := owner[k]; dup {
				p.add("%s: %s%s is already mounted by %s", at, m.Host, m.Path, prev)
			} else {
				owner[k] = at
			}
		}
	}
}

// allPooled reports whether every view of a site is built by a pool, whose
// result other members' mounts serve, so the site needs no mounts of its own.
func (s *Spec) allPooled(site string) bool {
	views := s.Sites[site].Views
	for v := range views {
		if s.PoolOf(site, v) == "" {
			return false
		}
	}
	return len(views) > 0
}

func (s *Spec) validateNav(p *problems) {
	for i, n := range s.Nav {
		at := fmt.Sprintf("nav[%d]", i)
		if n.Title == "" {
			p.add("%s: title is required", at)
		}
		switch {
		case strings.HasPrefix(n.Link, "site://"):
			name, _, _ := strings.Cut(strings.TrimPrefix(n.Link, "site://"), "/")
			if _, ok := s.Sites[name]; !ok {
				p.add("%s: link %q names no site", at, n.Link)
			}
		case strings.HasPrefix(n.Link, "mount://"):
			host, _, _ := strings.Cut(strings.TrimPrefix(n.Link, "mount://"), "/")
			if !s.isMounted(host) {
				p.add("%s: link %q: nothing is mounted on %s", at, n.Link, host)
			}
		case strings.HasPrefix(n.Link, "https://"):
		default:
			p.add("%s: link %q must be site://, mount:// or https://", at, n.Link)
		}
	}
}

func (s *Spec) isMounted(host string) bool {
	for _, site := range s.Sites {
		for _, m := range site.Mounts {
			if m.Host == host {
				return true
			}
		}
	}
	return false
}

func validAccess(a string) bool { return a == AccessPublic || a == AccessPrivate }

// validateAccessHost keeps private mounts on the private domain and public
// mounts off it, so a typo can't publish something meant to stay private.
func (s *Spec) validateAccessHost(p *problems, at string, m Mount) {
	onPrivate := s.IsPrivateHost(m.Host)
	switch {
	case m.Access == AccessPrivate && s.PrivateDomain == "":
		p.add("%s: access is private, but no private_domain is declared", at)
	case m.Access == AccessPrivate && !onPrivate:
		p.add("%s: private mounts must be on %s or a subdomain of it, not %q", at, s.PrivateDomain, m.Host)
	case m.Access == AccessPublic && onPrivate:
		p.add("%s: %q is on the private domain, so its access must be private", at, m.Host)
	}
}

// IsPrivateHost reports whether host is the private domain or under it.
func (s *Spec) IsPrivateHost(host string) bool {
	pd := s.PrivateDomain
	return pd != "" && (host == pd || strings.HasSuffix(host, "."+pd))
}

// DomainOf returns the declared domain host falls under, or "".
func (s *Spec) DomainOf(host string) string {
	for _, d := range s.Domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return d
		}
	}
	return ""
}

// sortedKeys gives map iteration a stable order, so error output and
// anything generated from the spec are deterministic.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
