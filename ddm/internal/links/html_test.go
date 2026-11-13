package links

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkRegions fails t unless the regions of data are in order, non-empty,
// inside data and apart, and every link found in them is one the whole-file
// pattern also finds: restricting where to look can only remove links.
func checkRegions(t *testing.T, data []byte) {
	t.Helper()
	prev := 0
	for _, s := range linkRegions(data) {
		if s.start < prev || s.end <= s.start || s.end > len(data) {
			t.Fatalf("bad region %v after offset %d in %q", s, prev, data)
		}
		prev = s.end
	}
	all := map[int]bool{}
	for _, m := range linkRE.FindAllSubmatchIndex(data, -1) {
		all[m[0]] = true
	}
	for _, m := range findLinks(data, true) {
		if !all[m[0]] {
			t.Fatalf("link at %d in %q is not found in the whole file", m[0], data)
		}
	}
}

const sampleDoc = `<!DOCTYPE html><?x y?><html lang=en><head><meta charset=utf-8>` +
	`<!-- ("site://nope/") --><!--><!---><style>a{background:url(site://cloakaml/l)}</style>` +
	`<SCRIPT type=module>if (a<b) u="site://cloakaml/"; s="</scripty>"</Script ></head>` +
	`<body><p title="a>b" data-x=a'b data-y='"' ="x" a= b = "c" d=>text (site://cloakaml/) < 3 <3 </ ` +
	`<code>[t](site://nope/)</code><a href='site://cloakaml/'>x</a><br/><svg><![CDATA[ > ]]></svg>` +
	`<textarea><a href="site://cloakaml/"></textarea></p a="b"></body></html>`

// TestLinkRegionsAnyPrefix cuts a document that uses every construct at every
// byte: the scanner must neither panic nor return nonsense for a file that
// ends anywhere, and a file that ends mid-construct is searched as before.
func TestLinkRegionsAnyPrefix(t *testing.T) {
	for n := 0; n <= len(sampleDoc); n++ {
		checkRegions(t, []byte(sampleDoc[:n]))
	}
}

func FuzzLinkRegions(f *testing.F) {
	f.Add(sampleDoc)
	f.Add(`<a href="site://cloakaml/">`)
	f.Add(`<script>"site://x/"`)
	f.Fuzz(func(t *testing.T, s string) { checkRegions(t, []byte(s)) })
}

// TestRewriteWhereLinksLive pins where a link is recognised in an HTML file:
// in a tag (its attribute values), and in the content of <script> and
// <style>; never in page text or in a comment. Every other file type is
// unchanged. In the tests, site://cloakaml/ resolves (to
// https://cloakaml.cathe.dev/) and site://nope/ names no site.
func TestRewriteWhereLinksLive(t *testing.T) {
	const L = "https://cloakaml.cathe.dev/" // what site://cloakaml/ becomes
	cases := []struct {
		name, file, in string
		want           string // "" means the file stays as written
		n              int    // links rewritten
		unresolved     bool   // the unresolved-link error is wanted
	}{
		// Examples of the syntax in page text: they are not links.
		{name: "example in code, target missing (the 2026-11-13 incident)", file: "a.html",
			in: `<p><code>[t](site://nope/x)</code></p>`},
		{name: "example in pre, target missing", file: "a.html",
			in: `<pre><code>[t](site://nope/x)</code></pre>`},
		{name: "example in code, target exists (the silent half)", file: "a.html",
			in: `<p><code>[t](site://cloakaml/)</code></p>`},
		{name: "bracket and quote in prose", file: "a.html",
			in: `<p>see (site://cloakaml/) or "site://cloakaml/" or 'site://nope/'</p>`},
		{name: "comment", file: "a.html",
			in: `<!-- [t](site://nope/) --><p>x</p>`},
		{name: ".htm is HTML too", file: "a.htm",
			in: `<code>[t](site://nope/)</code>`},
		{name: "CDATA is not a link position", file: "a.html",
			in: `<svg><![CDATA[ ("site://nope/") ]]></svg>`},

		// Links in tags.
		{name: "href, double and single quoted", file: "a.html",
			in:   `<a href="site://cloakaml/">x</a> <a href='site://cloakaml/'>y</a>`,
			want: `<a href="` + L + `">x</a> <a href='` + L + `'>y</a>`, n: 2},
		{name: "anchor nested in code", file: "a.html",
			in:   `<code><a href="site://cloakaml/">y</a></code>`,
			want: `<code><a href="` + L + `">y</a></code>`, n: 1},
		{name: "upper case tag and attribute", file: "a.html",
			in:   `<A HREF="site://cloakaml/">y</A>`,
			want: `<A HREF="` + L + `">y</A>`, n: 1},
		{name: "a > inside an earlier attribute value", file: "a.html",
			in:   `<a title="a > b" href="site://cloakaml/">x</a>`,
			want: `<a title="a > b" href="` + L + `">x</a>`, n: 1},
		{name: "an apostrophe in an unquoted value opens no quote", file: "a.html",
			in:   `<a data-x=a'b href="site://cloakaml/">x</a>`,
			want: `<a data-x=a'b href="` + L + `">x</a>`, n: 1},
		{name: "url() in a style attribute", file: "a.html",
			in:   `<div style="background:url(site://cloakaml/l.svg)">x</div>`,
			want: `<div style="background:url(` + L + `l.svg)">x</div>`, n: 1},
		{name: "attributes on a closing tag are still a tag", file: "a.html",
			in:   `<p>x</p a="site://cloakaml/">`,
			want: `<p>x</p a="` + L + `">`, n: 1},
		{name: "href unresolved is still an error", file: "a.html",
			in: `<a href="site://nope/">x</a>`, unresolved: true},

		// Text right after a tag is text again.
		{name: "text after a tag whose value holds a >", file: "a.html",
			in: `<a title="x>y">(site://cloakaml/)</a>`},
		{name: "text after a self-closing tag", file: "a.html",
			in: `<br/>(site://cloakaml/)`},

		// <script> and <style> are code: links live in them.
		{name: "style element", file: "a.html",
			in:   `<style>a{background:url(site://cloakaml/l.svg)}</style>`,
			want: `<style>a{background:url(` + L + `l.svg)}</style>`, n: 1},
		{name: "script string", file: "a.html",
			in:   `<script>var u = "site://cloakaml/";</script>`,
			want: `<script>var u = "` + L + `";</script>`, n: 1},
		{name: "script with a < in it", file: "a.html",
			in:   `<script>if (a < b) u = "site://cloakaml/"</script>`,
			want: `<script>if (a < b) u = "` + L + `"</script>`, n: 1},
		{name: "json script", file: "a.html",
			in:   `<script type="application/json">{"u":"site://cloakaml/"}</script>`,
			want: `<script type="application/json">{"u":"` + L + `"}</script>`, n: 1},
		{name: "upper-case script and closing tag", file: "a.html",
			in:   `<SCRIPT>u="site://cloakaml/"</Script ><p>"site://cloakaml/"</p>`,
			want: `<SCRIPT>u="` + L + `"</Script ><p>"site://cloakaml/"</p>`, n: 1},
		{name: "a string that only looks like the closing tag", file: "a.html",
			in:   `<script>s="</scripty>"; u="site://cloakaml/"</script>`,
			want: `<script>s="</scripty>"; u="` + L + `"</script>`, n: 1},
		{name: "text after script and after style", file: "a.html",
			in: `<script>x=1</script><p>"site://cloakaml/"</p><style>a{}</style><p>(site://cloakaml/)</p>`},

		// A file that ends mid-construct: no panic, and the rest is handled as
		// before, since it cannot be classified.
		{name: "ends in a tag", file: "a.html",
			in: `<p>x</p><a href="site://cloakaml/`, want: `<p>x</p><a href="` + L, n: 1},
		{name: "ends in a comment", file: "a.html",
			in: `<p>x</p><!-- ("site://cloakaml/")`, want: `<p>x</p><!-- ("` + L + `")`, n: 1},
		{name: "ends in a script", file: "a.html",
			in: `<script>u="site://cloakaml/"`, want: `<script>u="` + L + `"`, n: 1},
		{name: "ends in a declaration", file: "a.html",
			in: `<p>x</p><!DOCTYPE ("site://cloakaml/"`, want: `<p>x</p><!DOCTYPE ("` + L + `"`, n: 1},
		{name: "ends after a <", file: "a.html", in: `<p>(site://cloakaml/)</p><`},
		{name: "ends after </", file: "a.html", in: `<p>(site://cloakaml/)</p></`},
		{name: "ends after <!", file: "a.html", in: `<p>(site://cloakaml/)</p><!`},

		// Other file types are exactly as before: a link after a quote or "(" is a
		// link, wherever it sits.
		{name: "xml text", file: "a.xml",
			in: `<p>(site://cloakaml/)</p>`, want: `<p>(` + L + `)</p>`, n: 1},
		{name: "css", file: "a.css",
			in: `a{background:url(site://cloakaml/l.svg)}`, want: `a{background:url(` + L + `l.svg)}`, n: 1},
		{name: "js", file: "a.js",
			in: `var u = "site://cloakaml/"`, want: `var u = "` + L + `"`, n: 1},
		{name: "json", file: "a.json",
			in: `{"u":"site://cloakaml/"}`, want: `{"u":"` + L + `"}`, n: 1},
		{name: "svg text", file: "a.svg",
			in: `<text>("site://cloakaml/")</text>`, want: `<text>("` + L + `")</text>`, n: 1},
	}
	r := resolver(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, c.file)
			if err := os.WriteFile(path, []byte(c.in), 0o644); err != nil {
				t.Fatal(err)
			}
			n, err := r.Rewrite(dir, "forest", "public")
			if c.unresolved != (err != nil) || (err != nil && !strings.Contains(err.Error(), "unresolved links")) {
				t.Errorf("unresolved error: want %v, got %v", c.unresolved, err)
			}
			if n != c.n {
				t.Errorf("rewrote %d links, want %d", n, c.n)
			}
			want := c.want
			if want == "" {
				want = c.in
			}
			if got, _ := os.ReadFile(path); string(got) != want {
				t.Errorf("\n got %s\nwant %s", got, want)
			}
		})
	}
}
