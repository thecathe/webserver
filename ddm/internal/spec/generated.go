package spec

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DefaultGeneratedBy is who a `generated:` block names when it names no one.
const DefaultGeneratedBy = "Claude"

// Generated says a site's pages were written by an AI, so the shared marker
// (ddm/internal/aimark) is drawn on them. It sits on a site and covers every
// view of it. Model, Effort, Date and Edited are defaults: a page's own
// `generated` front matter, which the generator turns into a <meta> tag,
// overrides them, and records the dates a site-wide block can't know.
type Generated struct {
	By     string `yaml:"by"` // default "Claude"
	Model  string `yaml:"model"`
	Effort string `yaml:"effort"`
	Date   string `yaml:"date"`   // first generated, YYYY-MM-DD
	Edited string `yaml:"edited"` // last edited, YYYY-MM-DD
	// Regions narrows the marker to some pages, or to part of a page. Empty
	// means every page.
	Regions []Region `yaml:"regions"`
	// Exclude lists pages that are never marked, even inside a region.
	Exclude []string `yaml:"exclude"`
}

// Region is a set of pages, optionally limited to part of each page.
type Region struct {
	// Paths are globs on a page's path in the published tree, with no
	// leading slash: `*` stays within a directory, `**` crosses them. A
	// mkdocs page a/b.md is a/b/index.html; either that or the directory
	// (a/b, a/b/) matches.
	Paths []string `yaml:"paths"`
	// Selector is a CSS selector for the part of the page that is marked.
	// Empty means the whole page.
	Selector string `yaml:"selector"`
}

// Matcher answers, for one page, whether and where a Generated block applies.
type Matcher struct {
	regions []compiledRegion
	exclude []*regexp.Regexp
}

type compiledRegion struct {
	paths    []*regexp.Regexp
	selector string
}

// Compile checks a block's globs and dates and prepares it for matching.
func (g *Generated) Compile() (*Matcher, error) {
	m := &Matcher{}
	for i, r := range g.Regions {
		if len(r.Paths) == 0 {
			return nil, fmt.Errorf("regions[%d]: paths is required", i)
		}
		c := compiledRegion{selector: strings.TrimSpace(r.Selector)}
		for _, p := range r.Paths {
			re, err := globRE(p)
			if err != nil {
				return nil, fmt.Errorf("regions[%d].paths: %w", i, err)
			}
			c.paths = append(c.paths, re)
		}
		m.regions = append(m.regions, c)
	}
	for _, p := range g.Exclude {
		re, err := globRE(p)
		if err != nil {
			return nil, fmt.Errorf("exclude: %w", err)
		}
		m.exclude = append(m.exclude, re)
	}
	return m, nil
}

// For reports whether the page at rel (a slash-separated path of an HTML file
// in the published tree) is marked, and the selector of the part that is
// ("" for the whole page).
func (m *Matcher) For(rel string) (selector string, ok bool) {
	names := pageNames(rel)
	for _, re := range m.exclude {
		if matchesAny(re, names) {
			return "", false
		}
	}
	if len(m.regions) == 0 {
		return "", true
	}
	for _, r := range m.regions {
		for _, re := range r.paths {
			if matchesAny(re, names) {
				return r.selector, true
			}
		}
	}
	return "", false
}

// pageNames are the paths a glob may name a page by: the file itself and, for
// an index page, the directory it stands for (with and without a slash).
func pageNames(rel string) []string {
	names := []string{rel}
	switch rel[strings.LastIndex(rel, "/")+1:] {
	case "index.html", "index.htm":
		dir := strings.TrimSuffix(rel[:strings.LastIndex(rel, "/")+1], "/")
		if dir != "" {
			names = append(names, dir, dir+"/")
		}
	}
	return names
}

func matchesAny(re *regexp.Regexp, names []string) bool {
	for _, n := range names {
		if re.MatchString(n) {
			return true
		}
	}
	return false
}

// globRE turns `a/*/b`, `a/**` and `**/c.html` into an anchored expression.
func globRE(glob string) (*regexp.Regexp, error) {
	if glob == "" || strings.HasPrefix(glob, "/") || strings.Contains(glob, `\`) || strings.Contains(glob, "..") {
		return nil, fmt.Errorf("glob %q must be a path in the published tree: not empty, no leading /, no .. and no backslash", glob)
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()), nil
}

// validateGenerated reports problems with a site's `generated:` block.
func validateGenerated(p *problems, where string, g *Generated) {
	if g == nil {
		return
	}
	at := where + ".generated"
	if strings.TrimSpace(g.By) == "" {
		p.add("%s: by must not be empty", at)
	}
	for field, v := range map[string]string{"date": g.Date, "edited": g.Edited} {
		if v == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", v); err != nil {
			p.add("%s: %s %q must be a date like 2026-10-16", at, field, v)
		}
	}
	if _, err := g.Compile(); err != nil {
		p.add("%s: %v", at, err)
	}
}
