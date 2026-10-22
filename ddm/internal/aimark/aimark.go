// Package aimark is ddm's AI-generated marker: a yellow header that sticks
// while the page scrolls, and a pale yellow tint, drawn on pages (or on
// regions of pages) that say they were written by an AI.
//
// What is marked comes from two places, merged in the browser (ai.js):
//
//   - the site: sites.yaml `generated:` (spec.Generated) covers a site's
//     pages, or the regions it names. Publishing adds a <meta
//     name="ddm:generated-site"> to each covered page (PageHead);
//   - the page: a generator turns a page's own `generated` front matter into
//     <meta name="ddm:generated"> (scripts/mkdocs_ddm_generated.py for
//     mkdocs). It wins, and can opt the page out.
//
// Every published page also gets the stylesheet and script (Tags), so a
// hand-written site can mark a single page. Both files are served from
// /_ddm/ on every host, like the toolbar's. They contain nothing private.
package aimark

import (
	_ "embed"
	"html"
	"os"
	"path/filepath"
	"strings"

	"ddm/internal/bar"
	"ddm/internal/spec"
)

//go:embed ai.js
var script []byte

//go:embed ai.css
var style []byte

// Tags are added before </head> of every published page. Both are valid XML,
// for the forest's XSLT pages.
const Tags = `<link rel="stylesheet" href="/_ddm/ai.css"/><script type="module" src="/_ddm/ai.js"></script>`

// Write writes ai.css and ai.js for both audiences.
func Write(www string) error {
	for _, private := range []bool{false, true} {
		dir := bar.Dir(www, private)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "ai.js"), script, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "ai.css"), style, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// PageHead returns what publishing adds to the page at rel (a slash-separated
// path in the published tree) for a site's `generated:` block: a meta tag, or
// "" when the block doesn't cover the page. A nil block marks nothing.
func PageHead(g *spec.Generated) (func(rel string) string, error) {
	if g == nil {
		return func(string) string { return "" }, nil
	}
	m, err := g.Compile()
	if err != nil {
		return nil, err
	}
	return func(rel string) string {
		selector, ok := m.For(rel)
		if !ok {
			return ""
		}
		xsl := strings.EqualFold(filepath.Ext(rel), ".xsl")
		var b strings.Builder
		b.WriteString(`<meta name="ddm:generated-site" content="` + attr(g.By, xsl) + `"`)
		for _, f := range []struct{ name, value string }{
			{"model", g.Model}, {"effort", g.Effort}, {"date", g.Date}, {"edited", g.Edited}, {"selector", selector},
		} {
			if f.value != "" {
				b.WriteString(` data-` + f.name + `="` + attr(f.value, xsl) + `"`)
			}
		}
		b.WriteString("/>")
		return b.String()
	}, nil
}

// attr escapes v for a double-quoted attribute. In an XSLT stylesheet, a
// brace in a literal element's attribute starts an expression, so it is
// doubled.
func attr(v string, xsl bool) string {
	v = html.EscapeString(v)
	if xsl {
		v = strings.NewReplacer("{", "{{", "}", "}}").Replace(v)
	}
	return v
}
