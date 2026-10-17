// Package links resolves site://<name>/<path> links (the rewrite-links
// stage), and mount://<host>/<path> links, which name one mount exactly (ddm's
// generated pages use them to list every address). It runs at publish time, not as a workflow step, because the URL a
// link becomes depends on where the build is published (prod or a local
// preview), not on the build itself.
package links

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ddm/internal/spec"
)

// BaseURL turns a host into the URL it is served at, e.g.
// "https://cloakaml.cathe.dev" in prod.
type BaseURL func(host string) string

// Prod serves every host over https.
func Prod(host string) string { return "https://" + host }

// linkRE matches site://name[/path] and mount://host[/path]. The path stops
// at whitespace, quotes, brackets and parentheses, which end a URL in HTML
// attributes, XML, CSS url(...) and JSON strings alike.
var linkRE = regexp.MustCompile("(site|mount)://([a-z0-9][a-z0-9.-]*)(/[^\\s\"'`<>()]*)?")

// rewritable are the file types that can contain links.
var rewritable = map[string]bool{
	".html": true, ".htm": true, ".xml": true, ".xsl": true, ".css": true,
	".js": true, ".json": true, ".svg": true,
}

// Resolver turns a link found in one site's view into a URL.
type Resolver struct {
	Spec *spec.Spec
	Base BaseURL
}

// Resolve returns the URL for site://<target><path> written in site from,
// view view.
//
//   - The target's view of the same name is used if it has one (a private
//     build links to private copies), else its public view.
//   - The first mount of that view, in spec order, is the canonical address.
//   - URLs are always absolute: one build may be served on several hosts, so
//     a root-relative link could point at the wrong one.
//   - A build with any public mount may never link to a private mount, which
//     would publish a private hostname.
func (r Resolver) Resolve(from, view, target, path string) (string, error) {
	t, ok := r.Spec.Sites[target]
	if !ok {
		return "", fmt.Errorf("site://%s: no site named %q", target, target)
	}
	tview := view
	if _, ok := t.Views[tview]; !ok {
		tview = spec.DefaultView
	}
	var mount *spec.Mount
	for i := range t.Mounts {
		if t.Mounts[i].View == tview {
			mount = &t.Mounts[i]
			break
		}
	}
	if mount == nil {
		return "", fmt.Errorf("site://%s: site %s has no mount for view %s", target, target, tview)
	}
	if mount.Access == spec.AccessPrivate && r.hasPublicMount(from, view) {
		return "", fmt.Errorf("site://%s: %s (%s) is publicly served, but %s is private (%s)", target, from, view, target, mount.Host)
	}
	url := r.Base(mount.Host)
	if mount.Path != "/" {
		url += mount.Path
	}
	if path == "" {
		path = "/"
	}
	return url + path, nil
}

// ResolveMount returns the URL for mount://<host><path> written in site
// from, view view. host must be mounted by some site.
func (r Resolver) ResolveMount(from, view, host, path string) (string, error) {
	private, mounted := false, false
	for _, site := range r.Spec.Sites {
		for _, m := range site.Mounts {
			if m.Host == host {
				mounted = true
				private = private || m.Access == spec.AccessPrivate
			}
		}
	}
	if !mounted {
		return "", fmt.Errorf("mount://%s: nothing is mounted on %s", host, host)
	}
	if private && r.hasPublicMount(from, view) {
		return "", fmt.Errorf("mount://%s: %s (%s) is publicly served, but %s is private", host, from, view, host)
	}
	if path == "" {
		path = "/"
	}
	return r.Base(host) + path, nil
}

func (r Resolver) hasPublicMount(site, view string) bool {
	for _, m := range r.Spec.Sites[site].Mounts {
		if m.View == view && m.Access == spec.AccessPublic {
			return true
		}
	}
	return false
}

// Rewrite replaces every site:// and mount:// link in the text files under dir, in place.
// It returns how many links it rewrote. Links that can't be resolved are
// left as they are and reported together in the error.
func (r Resolver) Rewrite(dir, site, view string) (int, error) {
	n := 0
	var problems []string
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() || !rewritable[strings.ToLower(filepath.Ext(path))] {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil || (!bytes.Contains(data, []byte("site://")) && !bytes.Contains(data, []byte("mount://"))) {
			return err
		}
		changed := false
		out := linkRE.ReplaceAllFunc(data, func(m []byte) []byte {
			sub := linkRE.FindSubmatch(m)
			resolve := r.Resolve
			if string(sub[1]) == "mount" {
				resolve = r.ResolveMount
			}
			url, err := resolve(site, view, string(sub[2]), string(sub[3]))
			if err != nil {
				if msg := err.Error(); !seen[msg] {
					seen[msg] = true
					rel, _ := filepath.Rel(dir, path)
					problems = append(problems, fmt.Sprintf("%s: %s", rel, msg))
				}
				return m
			}
			n++
			changed = true
			return []byte(url)
		})
		if !changed {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(path, out, info.Mode().Perm())
	})
	if err != nil {
		return n, err
	}
	if len(problems) > 0 {
		return n, fmt.Errorf("unresolved links:\n  %s", strings.Join(problems, "\n  "))
	}
	return n, nil
}
