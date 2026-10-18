package workflow

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ddm/internal/spec"
)

// siteDirectory generates pages from the spec itself. Param `page`:
//
//   - "directory" (default): index.html listing every indexed, publicly
//     mounted site by taxon (sites.cathe.dev). With param `taxon`, only the
//     sites with that taxon, as one list (docs.cathe.dev). Param `title`
//     names the page.
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
		page, data = "directory", directoryData(s, param(a.Params, "taxon", ""), param(a.Params, "title", "Sites"))
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

type dirPage struct {
	Title, Taxon string
	Sections     []dirSection
}

func directoryData(s *spec.Spec, only, title string) dirPage {
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
	if only != "" {
		order = []string{only}
	}
	page := dirPage{Title: title, Taxon: only}
	for _, t := range order {
		if len(byTaxon[t]) > 0 {
			page.Sections = append(page.Sections, dirSection{t, byTaxon[t]})
		}
	}
	return page
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
	Pool                                string // the pool it is built in, if any
}

type mapHost struct {
	Host    string
	Private bool
	Mounts  []mapMount
}

type mapPage struct {
	Hosts                  []mapHost
	Unmounted              []mapMount // pool members served only through other members' mounts
	Sites, Public, Private int
}

func mapData(s *spec.Spec) mapPage {
	byHost := map[string][]mapMount{}
	var page mapPage
	for _, name := range s.SiteNames() {
		site := s.Sites[name]
		page.Sites++
		if len(site.Mounts) == 0 {
			for _, view := range slices.Sorted(maps.Keys(site.Views)) {
				pool := s.PoolOf(name, view)
				var at []string
				for _, m := range s.MountsServing(spec.PoolName(pool), view) {
					at = append(at, m.Host+strings.TrimSuffix(m.Path, "/"))
				}
				page.Unmounted = append(page.Unmounted, mapMount{
					Site: name, View: view, Workflow: site.Views[view].Workflow, Pool: pool,
					Repo: site.Repo, RepoURL: repoURL(site.Repo), Branch: site.Branch,
					Description: site.Description, Indexed: site.IsIndexed(), Address: strings.Join(at, ", "),
				})
			}
		}
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
				Pool: s.PoolOf(name, m.View),
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

// splitJS is the split view on the directory pages (D52): a site's link opens
// it in a pane beside the directory.
//
//go:embed split.js
var splitJS string

// The styles are deliberately minimal, with the docs site's gruvbox tokens
// copied in. The shared-styling design (D27) will replace them.
var pages = template.Must(template.New("").Funcs(template.FuncMap{
	"splitScript": func() template.JS { return template.JS(splitJS) },
}).Parse(`
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

{{define "split"}}<style>
/* The split view (split.js): this page on the left, a site in a pane on the right, a divider between. */
:root { --grip-bg: #d5c4a1; --grip-hi: #f9f5d7; --grip-lo: #928374; }
@media (prefers-color-scheme: dark) { :root { --grip-bg: #3c3836; --grip-hi: #665c54; --grip-lo: #1d2021; } }
#ddm-split { display: none; }
@media (min-width: 800px) {
  html.split { overflow: hidden; }
  html.split body { position: fixed; top: var(--top, 0); left: 0; bottom: 0; width: var(--left); max-width: none;
                    margin: 0; overflow: auto; }
  html.split.min body { visibility: hidden; }
  html.split #ddm-split { display: flex; position: fixed; top: var(--top, 0); right: 0; bottom: 0; left: var(--left); }
  html.dragging, html.dragging * { cursor: col-resize !important; user-select: none; }
  html.dragging #ddm-split iframe { pointer-events: none; }
}
#ddm-split iframe { flex: 1; min-width: 0; border: 0; background: var(--bg); }
#ddm-split .divider { flex: none; width: 1.9rem; display: flex; flex-direction: column; align-items: center;
  gap: .35rem; padding: .45rem 0; background: var(--grip-bg); border-left: 1px solid var(--grip-lo);
  border-right: 1px solid var(--grip-lo); box-shadow: inset 1px 0 0 var(--grip-hi), inset -1px 0 0 var(--grip-lo); }
#ddm-split .controls { display: flex; flex-direction: column; gap: .3rem; }
#ddm-split .controls > * { width: 1.4rem; height: 1.4rem; display: grid; place-items: center; padding: 0;
  font: 13px/1 system-ui, sans-serif; color: var(--fg); text-decoration: none; cursor: pointer;
  background: var(--bg); border: 1px solid var(--grip-lo); border-radius: 3px; box-shadow: 0 1px 0 var(--grip-lo); }
#ddm-split .controls > *:hover { color: var(--link); border-color: var(--link); }
#ddm-split .controls > *:active { transform: translateY(1px); box-shadow: none; }
/* A knurled grip, like a tool handle's: cross-hatching over a fine grit, bevelled edges. */
#ddm-split .grip { flex: 1; width: .85rem; margin-top: .35rem; cursor: col-resize; touch-action: none;
  border: 1px solid var(--grip-lo); border-radius: 3px;
  background:
    repeating-linear-gradient(45deg, var(--grip-lo) 0 1px, transparent 1px 4px),
    repeating-linear-gradient(-45deg, var(--grip-hi) 0 1px, transparent 1px 4px),
    url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='64' height='64'%3E%3Cfilter id='g'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='1.4' numOctaves='2'/%3E%3CfeColorMatrix values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 .45 0'/%3E%3C/filter%3E%3Crect width='64' height='64' filter='url(%23g)'/%3E%3C/svg%3E"),
    var(--grip-bg);
  box-shadow: inset 1px 1px 0 var(--grip-hi), inset -1px -1px 0 var(--grip-lo), 0 0 0 1px var(--grip-bg); }
#ddm-split .grip:hover, html.dragging #ddm-split .grip { border-color: var(--link); }
#ddm-split .grip:focus-visible, #ddm-split .controls > *:focus-visible { outline: 2px solid var(--link); outline-offset: 1px; }
</style>{{end}}

{{define "directory"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>{{template "style"}}{{template "split"}}</head><body>
<h1>{{.Title}}</h1>
<p class="muted">{{if .Taxon}}Every public site tagged <em>{{.Taxon}}</em>{{else}}Every public site, by topic{{end}}. Generated by ddm from <code>sites.yaml</code>.</p>
{{$one := .Taxon}}{{range .Sections}}{{if not $one}}<h2>{{.Taxon}}</h2>{{end}}<ul>
{{range .Sites}}<li><a href="site://{{.Name}}/" data-split>{{.Name}}</a>{{if .Description}}: {{.Description}}{{end}}</li>
{{end}}</ul>
{{end}}{{if not .Sections}}<p>Nothing here yet.</p>{{end}}
<script>{{splitScript}}</script>
</body></html>
{{end}}

{{define "map"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Site map</title>{{template "style"}}{{template "split"}}</head><body>
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
<td><a href="{{.Href}}" data-split>{{.Address}}</a>{{if .Entry}}<br><span class="muted">→ {{.Entry}}</span>{{end}}</td>
<td><span class="badge {{.Access}}">{{.Access}}</span></td>
<td><strong>{{.Site}}</strong>{{if .Description}}<br><span class="muted">{{.Description}}</span>{{end}}</td>
<td>{{.View}}</td>
<td>{{.Workflow}}</td>
<td>{{if .RepoURL}}<a href="{{.RepoURL}}">{{.Repo}}</a> <span class="muted">({{.Branch}})</span>{{else if .Repo}}{{.Repo}}{{else}}<span class="muted">generated</span>{{end}}</td>
<td>{{if .Pool}}built in the {{.Pool}} pool<br>{{end}}{{if not .Indexed}}<span class="muted">not in the directory</span>{{end}}</td>
</tr>
{{end}}</table></div>
</section>
{{end}}{{if .Unmounted}}<section>
<h2>Without an address of their own</h2>
<p class="muted">Built in a pool, and served through the pool's addresses.</p>
<div class="wrap"><table>
<tr><th>Site</th><th>Pool</th><th>Served at</th><th>Source</th></tr>
{{range .Unmounted}}<tr>
<td><strong>{{.Site}}</strong>{{if .Description}}<br><span class="muted">{{.Description}}</span>{{end}}</td>
<td>{{.Pool}}</td>
<td>{{.Address}}</td>
<td>{{if .RepoURL}}<a href="{{.RepoURL}}">{{.Repo}}</a> <span class="muted">({{.Branch}})</span>{{else}}{{.Repo}}{{end}}</td>
</tr>
{{end}}</table></div>
</section>
{{end}}<script>{{splitScript}}</script>
</body></html>
{{end}}

{{define "notfound"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Not found</title>{{template "style"}}</head><body>
<h1>Not found</h1>
<p>There is nothing at this address.</p>
<p id="redirect" hidden>Taking you to <a id="home" href="/">this site's home page</a> in 3 seconds.</p>
<p><a id="directory" href="site://sites/">Browse every site</a></p>
<script>
// Back to this host's home page (its / forwards to its entry page), or, if
// the host itself is unknown so / is missing too, to the site directory.
const target = location.pathname === "/" ? document.getElementById("directory") : document.getElementById("home");
if (location.pathname === "/") document.getElementById("home").replaceWith(document.createTextNode("the site directory"));
document.getElementById("redirect").hidden = false;
setTimeout(() => location.replace(target.href), 3000);
</script>
</body></html>
{{end}}
`))
