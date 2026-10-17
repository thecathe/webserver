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
	"os"
	"path/filepath"
	"strings"
	"time"

	"ddm/internal/build"
	"ddm/internal/links"
	"ddm/internal/spec"
)

//go:embed bar.js
var script []byte

// Script is the tag publishing adds before </head>. defer="defer" keeps it
// valid XML, for XSLT stylesheets such as the forest's.
const Script = `<script src="/_ddm/bar.js" defer="defer"></script>`

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
}

// Write writes bar.js and nav.json for both audiences.
func Write(www string, s *spec.Spec, base links.BaseURL) error {
	r := links.Resolver{Spec: s, Base: base}
	for _, private := range []bool{false, true} {
		var n nav
		for _, name := range s.SiteNames() {
			for _, m := range s.Sites[name].Mounts {
				if m.Access == spec.AccessPrivate && !private {
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
// always, and for public pages too if any public mount serves it.
func WriteStatus(www string, s *spec.Spec, st Status) error {
	public := false
	for _, m := range s.MountsServing(st.Served, st.View) {
		public = public || m.Access == spec.AccessPublic
	}
	for _, private := range []bool{false, true} {
		if !private && !public {
			continue
		}
		dir := filepath.Join(Dir(www, private), "status", st.View)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, st.Served+".json"), st); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
