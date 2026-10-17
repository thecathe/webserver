package workflow

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ddm/internal/spec"
)

// siteDirectory generates pages from the spec itself. Param `page`:
//
//   - "directory" (default): index.html listing every indexed, publicly
//     mounted site by taxon (docs.cathe.dev).
//   - "map": index.html listing every mount of every site, grouped by host,
//     each marked public or private (the private site map).
//
// Both also write the shared 404.html. Links are written as site:// and
// mount:// so that publishing resolves them for its target (prod or preview).
func siteDirectory(_ context.Context, a BuiltinArgs) error {
	s := a.Plan.Spec
	if s == nil {
		return fmt.Errorf("no spec in the plan")
	}
	var page string
	var data any
	switch kind := param(a.Params, "page", "directory"); kind {
	case "directory":
		page, data = "directory", directoryData(s)
	case "map":
		page, data = "map", mapData(s)
	default:
		return fmt.Errorf(`page %q must be "directory" or "map"`, kind)
	}
	out := a.Plan.Vars.Out
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for file, tmpl := range map[string]string{"index.html": page, "404.html": "notfound"} {
		f, err := os.Create(filepath.Join(out, file))
		if err != nil {
			return err
		}
		err = pages.ExecuteTemplate(f, tmpl, data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	fmt.Fprintf(a.Log, "    wrote the %s page and 404.html\n", page)
	return nil
}

type dirSite struct {
	Name, Description string
}

type dirSection struct {
	Taxon string
	Sites []dirSite
}

func directoryData(s *spec.Spec) []dirSection {
	byTaxon := map[string][]dirSite{}
	for _, name := range s.SiteNames() {
		site := s.Sites[name]
		if !site.IsIndexed() || !hasPublicMount(site) {
			continue
		}
		taxa := site.Taxa
		if len(taxa) == 0 {
			taxa = []string{"other"}
		}
		for _, t := range taxa {
			byTaxon[t] = append(byTaxon[t], dirSite{name, site.Description})
		}
	}
	order := slices.Clone(s.Taxa)
	for t := range byTaxon {
		if !slices.Contains(order, t) {
			order = append(order, t)
		}
	}
	var out []dirSection
	for _, t := range order {
		if len(byTaxon[t]) > 0 {
			out = append(out, dirSection{t, byTaxon[t]})
		}
	}
	return out
}

func hasPublicMount(site spec.Site) bool {
	for _, m := range site.Mounts {
		if m.View == spec.DefaultView && m.Access == spec.AccessPublic {
			return true
		}
	}
	return false
}

type mapMount struct {
	Href                                template.URL // mount://, resolved when published
	Address                             string
	Site, View, Workflow, Entry, Access string
	Repo, RepoURL, Branch, Description  string
	Indexed                             bool
}

type mapHost struct {
	Host    string
	Private bool
	Mounts  []mapMount
}

type mapPage struct {
	Hosts                  []mapHost
	Sites, Public, Private int
}

func mapData(s *spec.Spec) mapPage {
	byHost := map[string][]mapMount{}
	var page mapPage
	for _, name := range s.SiteNames() {
		site := s.Sites[name]
		page.Sites++
		for _, m := range site.Mounts {
			addr := m.Host
			if m.Path != "/" {
				addr += m.Path
			}
			mm := mapMount{
				Address: addr, Href: template.URL("mount://" + m.Host + strings.TrimSuffix(m.Path, "/") + "/"),
				Site: name, View: m.View, Workflow: site.Views[m.View].Workflow, Entry: m.Entry,
				Access: m.Access, Repo: site.Repo, RepoURL: repoURL(site.Repo), Branch: site.Branch,
				Description: site.Description, Indexed: site.IsIndexed(),
			}
			if m.Access == spec.AccessPrivate {
				page.Private++
			} else {
				page.Public++
			}
			byHost[m.Host] = append(byHost[m.Host], mm)
		}
	}
	hosts := make([]string, 0, len(byHost))
	for h := range byHost {
		hosts = append(hosts, h)
	}
	// Grouped by domain (reversed labels), so subdomains sit under their parent.
	slices.SortFunc(hosts, func(a, b string) int { return strings.Compare(reverseHost(a), reverseHost(b)) })
	for _, h := range hosts {
		ms := byHost[h]
		slices.SortFunc(ms, func(a, b mapMount) int { return strings.Compare(a.Address, b.Address) })
		page.Hosts = append(page.Hosts, mapHost{Host: h, Private: s.IsPrivateHost(h), Mounts: ms})
	}
	return page
}

func reverseHost(h string) string {
	labels := strings.Split(h, ".")
	slices.Reverse(labels)
	return strings.Join(labels, ".")
}

func repoURL(repo string) string {
	switch {
	case repo == "":
		return ""
	case strings.Contains(repo, "://"):
		return repo
	case !strings.HasPrefix(repo, "/") && strings.Count(repo, "/") == 1:
		return "https://github.com/" + repo
	}
	return ""
}

// The styles are deliberately minimal, with the docs site's gruvbox tokens
// copied in. The shared-styling design (D27) will replace them.
var pages = template.Must(template.New("").Parse(`
{{define "style"}}<style>
:root { --bg: #fbf1c7; --fg: #3c3836; --muted: #7c6f64; --line: #d5c4a1; --link: #af3a03;
        --public: #79740e; --public-bg: #f2e5bc; --private: #9d0006; --private-bg: #f9d9c9; }
@media (prefers-color-scheme: dark) {
  :root { --bg: #1d2021; --fg: #ebdbb2; --muted: #a89984; --line: #504945; --link: #fe8019;
          --public: #b8bb26; --public-bg: #32361a; --private: #fb4934; --private-bg: #3c1f1e; }
}
* { box-sizing: border-box; }
body { margin: 0 auto; max-width: 64rem; padding: 2rem 1rem; background: var(--bg); color: var(--fg);
       font: 16px/1.5 system-ui, sans-serif; }
a { color: var(--link); }
h1 { margin-top: 0; } h2 { margin-top: 2rem; border-bottom: 1px solid var(--line); }
.muted { color: var(--muted); }
table { width: 100%; border-collapse: collapse; font-size: .95rem; }
th, td { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid var(--line); vertical-align: top; }
th { font-weight: 600; color: var(--muted); }
.badge { display: inline-block; padding: 0 .5rem; border-radius: 1rem; font-size: .8rem; font-weight: 600;
         border: 1px solid currentColor; white-space: nowrap; }
.badge.public { color: var(--public); background: var(--public-bg); }
.badge.private { color: var(--private); background: var(--private-bg); }
.badge.private::before { content: "🔒 "; }
.host-private h2 { border-bottom-color: var(--private); }
.wrap { overflow-x: auto; }
</style>{{end}}

{{define "directory"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sites</title>{{template "style"}}</head><body>
<h1>Sites</h1>
<p class="muted">Every public site, by topic. Generated by ddm from <code>sites.yaml</code>.</p>
{{range .}}<h2>{{.Taxon}}</h2><ul>
{{range .Sites}}<li><a href="site://{{.Name}}/">{{.Name}}</a>{{if .Description}}: {{.Description}}{{end}}</li>
{{end}}</ul>
{{end}}</body></html>
{{end}}

{{define "map"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Site map</title>{{template "style"}}</head><body>
<h1>Site map</h1>
<p class="muted">Everything ddm serves: {{.Sites}} sites,
<span class="badge public">public</span> {{.Public}} mounts,
<span class="badge private">private</span> {{.Private}} mounts.
Generated from <code>sites.yaml</code> on every build, so anything newly deployed appears here.</p>
{{range .Hosts}}<section class="{{if .Private}}host-private{{else}}host-public{{end}}">
<h2>{{.Host}}</h2>
<div class="wrap"><table>
<tr><th>Address</th><th>Access</th><th>Site</th><th>View</th><th>Workflow</th><th>Source</th><th>Notes</th></tr>
{{range .Mounts}}<tr>
<td><a href="{{.Href}}">{{.Address}}</a>{{if .Entry}}<br><span class="muted">→ {{.Entry}}</span>{{end}}</td>
<td><span class="badge {{.Access}}">{{.Access}}</span></td>
<td><strong>{{.Site}}</strong>{{if .Description}}<br><span class="muted">{{.Description}}</span>{{end}}</td>
<td>{{.View}}</td>
<td>{{.Workflow}}</td>
<td>{{if .RepoURL}}<a href="{{.RepoURL}}">{{.Repo}}</a> <span class="muted">({{.Branch}})</span>{{else if .Repo}}{{.Repo}}{{else}}<span class="muted">generated</span>{{end}}</td>
<td>{{if not .Indexed}}<span class="muted">not in the directory</span>{{end}}</td>
</tr>
{{end}}</table></div>
</section>
{{end}}</body></html>
{{end}}

{{define "notfound"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Not found</title>{{template "style"}}</head><body>
<h1>Not found</h1>
<p>There is nothing at this address.</p>
<p><a href="site://docs/">Browse every site</a></p>
</body></html>
{{end}}
`))
