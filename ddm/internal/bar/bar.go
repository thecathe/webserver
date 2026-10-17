// Package bar is ddm's toolbar: one small script, added to every published
// page, that draws the pillar links (sites.yaml `nav:`), a status line (what
// was built, from which repo, branch and commit) and links to the same page
// in the site's other views (private ↔ public ↔ notes).
//
// The script reads JSON that ddm writes beside it:
//
//	<www>/_ddm/public/   bar.js, nav.json (public mounts and links only), status/
//	<www>/_ddm/private/  the same, with private mounts and links too
//
// Caddy serves /_ddm/ on every host from the matching directory, so a public
// page never learns a private address. The look is a placeholder until the
// shared-styling design (D27).
package bar

import (
	_ "embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ddm/internal/build"
	"ddm/internal/links"
	"ddm/internal/spec"
)

//go:embed bar.js
var script []byte

// Script is the tag publishing adds before </head>. A module script, like
// the forest's own forester.js: it runs after the page is parsed, including
// pages that an XSLT stylesheet builds in the browser, and is valid XML there.
const Script = `<script type="module" src="/_ddm/bar.js"></script>`

// Dir is where the toolbar's files for one audience live.
func Dir(www string, private bool) string {
	if private {
		return filepath.Join(www, "_ddm", "private")
	}
	return filepath.Join(www, "_ddm", "public")
}

type mount struct {
	Origin string `json:"origin"` // scheme://host[:port], as the browser sees it
	Path   string `json:"path"`
	Site   string `json:"site"`
	View   string `json:"view"`
	Served string `json:"served"`
	Access string `json:"access"`
}

type link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type nav struct {
	Links  []link  `json:"links"`
	Mounts []mount `json:"mounts"`
	// Preview marks a local preview: public pages then also get a button to
	// the private view, for testing. Never written for production, where it
	// would publish the private addresses.
	Preview bool `json:"preview,omitempty"`
}

// Write writes bar.js and nav.json for both audiences. With preview, the
// public nav.json carries the private mounts too, for the testing button.
func Write(www string, s *spec.Spec, base links.BaseURL, preview bool) error {
	r := links.Resolver{Spec: s, Base: base}
	for _, private := range []bool{false, true} {
		n := nav{Preview: preview}
		for _, name := range s.SiteNames() {
			for _, m := range s.Sites[name].Mounts {
				if m.Access == spec.AccessPrivate && !private && !preview {
					continue
				}
				n.Mounts = append(n.Mounts, mount{
					Origin: base(m.Host), Path: m.Path, Site: name, View: m.View,
					Served: s.Served(name, m.View), Access: m.Access,
				})
			}
		}
		for _, l := range s.Nav {
			url, host, ok := resolve(r, l.Link, private)
			if !ok || (s.IsPrivateHost(host) && !private) {
				continue
			}
			n.Links = append(n.Links, link{l.Title, url})
		}
		dir := Dir(www, private)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "bar.js"), script, 0o644); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, "nav.json"), n); err != nil {
			return err
		}
	}
	return nil
}

// resolve turns a nav link into a URL and the host it points at.
func resolve(r links.Resolver, l string, private bool) (url, host string, ok bool) {
	view := spec.DefaultView
	if private {
		view = "private"
	}
	var err error
	switch {
	case strings.HasPrefix(l, "site://"):
		name, path, _ := strings.Cut(strings.TrimPrefix(l, "site://"), "/")
		url, err = r.Resolve("", view, name, "/"+path)
	case strings.HasPrefix(l, "mount://"):
		h, path, _ := strings.Cut(strings.TrimPrefix(l, "mount://"), "/")
		url, err = r.ResolveMount("", view, h, "/"+path)
	default:
		return l, "", true
	}
	if err != nil {
		return "", "", false
	}
	rest := url[strings.Index(url, "://")+3:]
	host, _, _ = strings.Cut(rest, "/")
	host, _, _ = strings.Cut(host, ".localhost") // a preview host
	host, _, _ = strings.Cut(host, ":")
	return url, host, true
}

// Status is what one publish was built from.
type Status struct {
	Served    string         `json:"served"`
	View      string         `json:"view"`
	Commit    string         `json:"commit"`
	Published time.Time      `json:"published"`
	Members   []build.Member `json:"members"`
}

// WriteStatus records a publish for the status line: for private pages
// always, and for public pages too if any public mount serves it. The
// public copy leaves out the repo, branch and commit of any repo that isn't
// public, so a public page never links to a private repo.
func WriteStatus(www string, s *spec.Spec, st Status) error {
	public := false
	for _, m := range s.MountsServing(st.Served, st.View) {
		public = public || m.Access == spec.AccessPublic
	}
	for _, private := range []bool{false, true} {
		if !private && !public {
			continue
		}
		out := st
		if !private {
			out.Members = nil
			for _, m := range st.Members {
				if m.Repo != "" && !RepoIsPublic(m.Repo) {
					m = build.Member{Site: m.Site, Packages: m.Packages}
				}
				out.Members = append(out.Members, m)
			}
		}
		dir := filepath.Join(Dir(www, private), "status", st.View)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, st.Served+".json"), out); err != nil {
			return err
		}
	}
	return nil
}

// RepoIsPublic reports whether GitHub serves repo ("owner/name") to anyone.
// Anything else, including a failed check, counts as private: the cost of a
// wrong "private" is a missing link; of a wrong "public", a leaked repo name.
// Replaceable in tests.
var RepoIsPublic = func(repo string) bool {
	repoMu.Lock()
	defer repoMu.Unlock()
	if v, ok := repoCache[repo]; ok {
		return v
	}
	ok := false
	if repoRE.MatchString(repo) {
		c := http.Client{Timeout: 10 * time.Second}
		if resp, err := c.Head("https://github.com/" + repo); err == nil {
			resp.Body.Close()
			// An anonymous request: a private repo is a 404 to us.
			ok = resp.StatusCode == http.StatusOK
		}
	}
	repoCache[repo] = ok
	return ok
}

var (
	repoMu    sync.Mutex
	repoCache = map[string]bool{}
	repoRE    = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

// WritePaths lists the pages of a public build (every .html file, as a URL
// path) for private pages, so their "public" button can tell whether this
// page has a public version. Private-only: the list is of public pages, but
// it is written beside private data and never served publicly.
func WritePaths(www, view, served, dir string) error {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		paths = append(paths, "/"+filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		return err
	}
	out := filepath.Join(Dir(www, true), "paths", view)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(out, served+".json"), paths)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
