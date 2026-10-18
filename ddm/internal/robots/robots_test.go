package robots

import (
	"strings"
	"testing"

	"ddm/internal/spec"
)

var r = spec.Robots{Training: []string{"GPTBot"}, Assistants: []string{"Claude-User"}}

// group returns the rules after the line "User-agent: <ua>", up to the next
// blank line.
func group(txt, ua string) string {
	_, rest, ok := strings.Cut(txt, "User-agent: "+ua+"\n")
	if !ok {
		return ""
	}
	g, _, _ := strings.Cut(rest, "\n\n")
	return strings.TrimSuffix(g, "\n")
}

func TestEntryOnly(t *testing.T) {
	txt := Text("jpears.me", r, []spec.Mount{{Path: "/", Entry: "/@jpears/", Search: spec.SearchEntry}}, false)
	if g := group(txt, "GPTBot"); g != "Disallow: /" {
		t.Errorf("training crawlers must be refused, got %q", g)
	}
	if g := group(txt, "Claude-User"); g != "Disallow: /" {
		t.Errorf("assistants must be refused without assistants: true, got %q", g)
	}
	all := group(txt, "*")
	for _, want := range []string{"search=yes, ai-input=no, ai-train=no", "Allow: /$", "Allow: /@jpears/$", "Allow: /@jpears/index.html$", "Allow: /*.css$", "Disallow: /"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "Allow: /\n") {
		t.Errorf("an entry-only host must not allow everything:\n%s", all)
	}
}

func TestDocsOpenToAssistants(t *testing.T) {
	txt := Text("ocaml.cathe.dev", r, []spec.Mount{{Path: "/", Search: spec.SearchAll, Assistants: true}}, false)
	if g := group(txt, "Claude-User"); g != "Allow: /" {
		t.Errorf("assistants: got %q", g)
	}
	if g := group(txt, "*"); !strings.HasSuffix(g, "Allow: /") || !strings.Contains(g, "ai-input=yes") {
		t.Errorf("search engines: got %q", g)
	}
}

func TestMixedHost(t *testing.T) {
	ms := []spec.Mount{
		{Path: "/", Search: spec.SearchAll},
		{Path: "/guide", Search: spec.SearchNone, Assistants: true},
	}
	txt := Text("docs.cathe.dev", r, ms, false)
	if g := group(txt, "Claude-User"); g != "Allow: /guide/\nDisallow: /" {
		t.Errorf("assistants on one mount: got %q", g)
	}
	if g := group(txt, "*"); !strings.Contains(g, "Allow: /\n") || !strings.HasSuffix(g, "Disallow: /") {
		t.Errorf("search: got %q", g)
	}
}

func TestNothingByDefault(t *testing.T) {
	txt := Text("demos.cathe.dev", r, []spec.Mount{{Path: "/", Search: spec.SearchNone}}, false)
	if g := group(txt, "*"); g != "Content-Signal: search=no, ai-input=no, ai-train=no\nDisallow: /" {
		t.Errorf("got %q", g)
	}
}

func TestPrivate(t *testing.T) {
	txt := Text("the.cathe.dev", r, []spec.Mount{{Path: "/", Search: spec.SearchAll, Assistants: true}}, true)
	if strings.Contains(txt, "Allow") || !strings.HasSuffix(txt, "Disallow: /\n") {
		t.Errorf("a private host must refuse everything:\n%s", txt)
	}
}
